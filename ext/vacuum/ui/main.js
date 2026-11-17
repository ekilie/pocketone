// Vacuum on Settings → Backups (PocketOne ext/vacuum): when the app's databases were last compacted, and the schedule.
// The status is vacuum/v1; the schedule is vacuum's settings/v1 section. When the app's host compacts its databases
// (vacuum/v1 answered by the host), the section is read-only: the host decides.
// Served at /_/extensions/vacuum/main.js. Plain DOM plus the dashboard's app.pb client.

(() => {
    const el = (tag, props = {}, ...children) => {
        const node = document.createElement(tag);
        for (const [k, v] of Object.entries(props)) {
            if (k === "className") node.className = v;
            else if (k.startsWith("on")) node.addEventListener(k.slice(2), v);
            else node.setAttribute(k, v);
        }
        for (const c of children.flat()) {
            if (c == null || c === false) continue;
            node.append(c instanceof Node ? c : document.createTextNode(String(c)));
        }
        return node;
    };
    const size = (b) => (b < 1 << 20 ? `${Math.max(0, Math.round(b / 1024))} KiB` : `${(b / (1 << 20)).toFixed(1)} MiB`);
    const when = (t) => (t ? new Date(t).toLocaleString() : "");
    const every = { "24h": "Every day", "168h": "Every week", "720h": "Every 30 days" };

    function lastRun(last) {
        if (!last) return "Not run yet.";
        if (last.error) return `Last run ${when(last.at)} failed: ${last.error}`;
        const skipped = last.skipped?.length ? ` Skipped (not enough free space to be worth it): ${last.skipped.join(", ")}.` : "";
        return `Last run ${when(last.at)}: ${size(last.saved || 0)} given back to the disk.${skipped}`;
    }

    function scheduleForm(st, section) {
        const enabled = el("input", { type: "checkbox", name: "enabled" });
        enabled.checked = !!st.enabled;
        const often = el("select", { name: "every" },
            ...Object.entries(every).map(([v, label]) => {
                const o = el("option", { value: v }, label);
                if (v === st.every) o.selected = true;
                return o;
            }));
        if (st.every && !every[st.every]) often.append(el("option", { value: st.every, selected: "" }, `Every ${st.every}`));
        const msg = el("div", { className: "m-t-sm" });
        const show = (cls, text) => msg.replaceChildren(el("div", { className: `alert ${cls}` }, text));
        const form = el("form", { className: "m-t-sm" },
            el("label", { className: "flex gap-10 m-b-sm" }, enabled, el("span", {}, "Compact the databases on a schedule")),
            el("label", { className: "flex gap-10 m-b-sm" }, el("span", { style: "min-width:9rem" }, "How often"), often),
            el("p", { className: "txt-hint" },
                "A database is only compacted when at least 10% of it (or 64 MB) is free space. Writes wait while it runs."),
            el("div", { className: "flex gap-10" }, el("button", { type: "submit", className: "btn sm" }, "Save")),
            msg);
        form.addEventListener("submit", async (e) => {
            e.preventDefault();
            try {
                await app.pb.send("/api/_c/settings/v1/save", {
                    method: "POST", requestKey: null,
                    body: { extension: "vacuum", values: { enabled: enabled.checked, every: often.value } },
                });
                render(section);
            } catch (err) {
                show("danger", err?.response?.message || err?.message || String(err));
            }
        });
        return form;
    }

    async function render(section) {
        let st;
        try {
            st = await app.pb.send("/api/_c/vacuum/v1", { requestKey: null });
        } catch (_) {
            section.remove(); // no vacuum on this app
            return;
        }
        const byHost = st.provider !== "vacuum";
        section.replaceChildren(
            el("div", { className: "txt-lg m-b-sm" }, "Vacuum"),
            el("p", { className: "txt-hint" },
                byHost
                    ? `Your host compacts this app's databases${st.every ? `, ${(every[st.every] || `every ${st.every}`).toLowerCase()}` : ""}. `
                    : st.enabled
                        ? `${every[st.every] || `Every ${st.every}`}${st.next ? `; next run ${when(st.next)}` : ""}. `
                        : "Off. ",
                lastRun(st.last)),
            byHost ? null : scheduleForm(st, section),
        );
    }

    document.addEventListener("mount:pageBackupsSettings", (e) => {
        const grid = e.detail.querySelector(".page-content .wrapper .grid") || e.detail;
        const section = el("div", { className: "col-lg-12 vacuum-section" });
        grid.append(section);
        render(section);
    });
})();
