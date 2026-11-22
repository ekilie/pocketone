// Settings → LLM for the PocketBase dashboard (PocketOne ext/llm): the providers $llm calls name as provider/model,
// each with its endpoint and key. Loaded as a UI extension (/_/extensions/llm/main.js). A view over llm's settings/v1
// section.
(() => {
    const SYSTEM = "System";
    app.store.settingsNavGroups[SYSTEM] = [
        ...(app.store.settingsNavGroups[SYSTEM] || []),
        { href: "#/settings/llm", icon: "ri-sparkling-2-line", label: "LLM" },
    ];

    const settingsAPI = (path, body) =>
        app.pb.send("/api/_c/settings/v1" + path, body ? { method: "POST", body, requestKey: null } : { requestKey: null });

    function sidebar() {
        return app.components.pageSidebar(
            { className: "settings-sidebar" },
            t.nav({ className: "sidebar-content scrollable" }, () => {
                const out = [];
                for (const group in app.store.settingsNavGroups) {
                    out.push(t.details(
                        { className: "nav-group", open: true },
                        t.summary({ tabIndex: -1, onfocusout: () => false, onclick: () => false, onkeyup: () => false }, group),
                        ...app.store.settingsNavGroups[group].map((link) => t.a(
                            { href: link.href, className: () => `nav-item ${app.utils.isActivePath(link.href, false) ? "active" : ""}` },
                            link.icon ? t.i({ className: link.icon, ariaHidden: true }) : null,
                            t.span({ className: "txt" }, link.label),
                        )),
                    ));
                }
                return out;
            }),
        );
    }

    const errText = (err) => err?.response?.message || err?.message || String(err);
    app.routes.superuserOnly("#/settings/llm", () => {
        app.store.title = "LLM";
        const data = store({
            loading: true,
            saving: false,
            view: null, // the section's settings/v1 values
            providers: [], // editable: { name, type, endpoint, account, key, keySet }
            adding: "", // the type picked in "Add a provider"
            message: null, // { kind: "success" | "danger" | "info", text }
        });

        function fill(v) {
            data.view = v;
            data.providers = (v.providers || []).map((p) => ({
                name: p.name, type: p.type, endpoint: p.endpoint || "", account: p.account || "", key: "", keySet: p.keySet,
            }));
        }

        async function load() {
            try {
                fill((await settingsAPI("/get?extension=llm")).values);
            } catch (err) {
                app.checkApiError(err);
            }
            data.loading = false;
        }

        function values() {
            return {
                providers: data.providers.map((p) => ({
                    name: p.name.trim(), type: p.type, endpoint: p.endpoint.trim(), account: p.account.trim(), key: p.key,
                })),
            };
        }

        async function save(e) {
            e?.preventDefault();
            data.saving = true;
            data.message = null;
            try {
                fill((await settingsAPI("/save", { extension: "llm", values: values() })).values);
                data.message = { kind: "success", text: "Saved." };
            } catch (err) {
                data.message = { kind: "danger", text: errText(err) };
            }
            data.saving = false;
        }

        async function test() {
            data.message = { kind: "info", text: "Testing…" };
            try {
                const r = await settingsAPI("/test", { extension: "llm", values: values() });
                data.message = r.ok ? { kind: "success", text: "Every provider answered with its key." } : { kind: "danger", text: r.message };
            } catch (err) {
                data.message = { kind: "danger", text: errText(err) };
            }
        }

        // set replaces one provider (so the page redraws), merging changes.
        const set = (i, changes) => (data.providers = data.providers.map((p, j) => (j === i ? { ...p, ...changes } : p)));

        function add() {
            const ti = (data.view?.types || []).find((x) => x.type === data.adding);
            if (!ti) return;
            let name = ti.name;
            for (let n = 2; data.providers.some((p) => p.name === name); n++) name = ti.name + n;
            data.providers = [...data.providers, { name, type: ti.type, endpoint: "", account: "", key: "", keySet: false }];
            data.adding = "";
        }

        function remove(i) {
            data.providers = data.providers.filter((_, j) => j !== i);
        }

        function field(label, input, hint) {
            return t.div({ className: "field m-b-sm" }, t.label({}, label), input, hint ? t.p({ className: "txt-hint m-t-5" }, hint) : null);
        }

        function providerCard(p, i) {
            const ti = (data.view?.types || []).find((x) => x.type === p.type) || {};
            return t.div({ className: "panel m-b-base", style: "padding:16px" },
                t.div({ className: "flex gap-sm m-b-sm", style: "align-items:center" },
                    t.strong({}, ti.title || p.type),
                    t.span({ className: "txt-hint" }, `${p.name}/…`),
                    t.span({ className: "flex-fill" }),
                    t.button({ type: "button", className: "btn transparent sm danger", onclick: () => remove(i) }, "Remove")),
                t.p({ className: "txt-hint m-b-sm" }, ti.about || ""),
                t.div({ className: "grid" },
                    t.div({ className: "col-lg-4" }, field("Name",
                        t.input({ type: "text", required: true, value: p.name, onchange: (e) => set(i, { name: e.target.value }) }),
                        "Calls name its models as name/model.")),
                    ti.account
                        ? t.div({ className: "col-lg-8" }, field("Account ID",
                            t.input({ type: "text", value: p.account, placeholder: "32 characters, from the dashboard URL", onchange: (e) => set(i, { account: e.target.value }) })))
                        : t.div({ className: "col-lg-8" }, field("Endpoint",
                            t.input({ type: "url", value: p.endpoint, placeholder: ti.endpoint || "https://…/v1", onchange: (e) => set(i, { endpoint: e.target.value }) }),
                            "Leave empty for the default; include the version path (/v1)."))),
                field(ti.account ? "API token" : "Key",
                    t.input({ type: "password", autocomplete: "off", value: p.key, placeholder: p.keySet ? "saved (leave empty to keep it)" : "",
                        onchange: (e) => set(i, { key: e.target.value }) })),
            );
        }

        load();

        return t.div(
            { className: "page page-llm" },
            sidebar(),
            t.div(
                { className: "page-content full-height" },
                t.header({ className: "page-header" },
                    t.nav({ className: "breadcrumbs" },
                        t.div({ className: "breadcrumb-item" }, "Settings"),
                        t.div({ className: "breadcrumb-item" }, "LLM"))),
                t.div(
                    { className: "wrapper m-b-base" },
                    t.p({ className: "txt-hint m-b-base" },
                        "The cloud providers behind ", t.code({}, "$llm"), " in your hooks and pages, each with your own key. ",
                        "A call names its model as ", t.code({}, "provider/model"), ": ", t.code({}, "vultr/glm-5.2"),
                        " uses the provider named ", t.code({}, "vultr"), " below. Each kind's ", t.code({}, "models()"),
                        " lists what the providers offer."),
                    () => data.loading
                        ? t.div({ className: "block txt-center" }, t.span({ className: "loader lg" }))
                        : t.form({ onsubmit: save },
                            t.h6({ className: "m-b-sm" }, "Providers"),
                            () => data.providers.length ? data.providers.map(providerCard) : t.p({ className: "txt-hint m-b-sm" }, "None yet."),
                            t.div({ className: "flex gap-sm m-b-base", style: "align-items:center" },
                                t.select({ onchange: (e) => (data.adding = e.target.value), style: "max-width:280px" },
                                    t.option({ value: "", selected: () => data.adding === "" }, "Add a provider…"),
                                    ...(data.view?.types || []).map((ti) => t.option({ value: ti.type, selected: () => data.adding === ti.type }, ti.title))),
                                t.button({ type: "button", className: "btn secondary", disabled: () => !data.adding, onclick: add }, "Add")),
                            () => data.message
                                ? t.div({ className: `alert ${data.message.kind} m-b-sm` }, t.div({ className: "content" }, data.message.text))
                                : null,
                            t.div({ className: "flex gap-sm" },
                                t.button({ type: "submit", className: "btn", disabled: () => data.saving }, "Save"),
                                t.button({ type: "button", className: "btn secondary", onclick: test }, "Test")),
                        ),
                ),
                t.footer({ className: "page-footer" }, app.components.credits()),
            ),
        );
    });
})();
