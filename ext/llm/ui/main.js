// Settings → LLM for the PocketBase dashboard (PocketOne ext/llm): the providers $llm calls, their models, and each
// kind's default. Loaded as a UI extension (/_/extensions/llm/main.js). A view over llm's settings/v1 section.
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

    const kindLabel = { text: "Text", image: "Images", speech: "Speech", transcribe: "Transcription", music: "Music", video: "Video", embed: "Embeddings" };
    const errText = (err) => err?.response?.message || err?.message || String(err);
    const price = (m) => (m.prices || []).slice(0, 2).map((p) => `$${+p.usd.toPrecision(3)} / ${p.unit}`).join(", ");

    app.routes.superuserOnly("#/settings/llm", () => {
        app.store.title = "LLM";
        const data = store({
            loading: true,
            saving: false,
            view: null, // the section's settings/v1 values
            providers: [], // editable: { name, type, endpoint, account, key, keySet, allModels, models, catalog, error, open, filter }
            defaults: {},
            adding: "", // the type picked in "Add a provider"
            message: null, // { kind: "success" | "danger" | "info", text }
        });

        function fill(v) {
            const open = Object.fromEntries(data.providers.map((p) => [p.name, p.open]));
            data.view = v;
            data.providers = (v.providers || []).map((p) => ({
                name: p.name, type: p.type, endpoint: p.endpoint || "", account: p.account || "", key: "", keySet: p.keySet,
                allModels: !!p.allModels, models: (p.catalog || []).filter((m) => m.enabled).map((m) => m.id),
                catalog: p.catalog || [], error: p.error || "", open: !!open[p.name], filter: "",
            }));
            data.defaults = { ...(v.defaults || {}) };
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
                    allModels: p.allModels, models: p.allModels ? [] : p.models,
                })),
                defaults: data.defaults,
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
                data.message = r.ok
                    ? { kind: "success", text: data.defaults.text ? "Every provider answered, and so did the text default." : "Every provider answered." }
                    : { kind: "danger", text: r.message };
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
            data.providers = [...data.providers, {
                name, type: ti.type, endpoint: "", account: "", key: "", keySet: false, allModels: true, models: [], catalog: [],
                error: "", open: true, filter: "",
            }];
            data.adding = "";
        }

        function remove(i) {
            const name = data.providers[i].name;
            data.providers = data.providers.filter((_, j) => j !== i);
            data.defaults = Object.fromEntries(Object.entries(data.defaults).filter(([, ref]) => !ref.startsWith(name + "/")));
        }

        function toggleModel(i, id, on) {
            const p = data.providers[i];
            const models = p.allModels ? p.catalog.map((m) => m.id) : [...p.models];
            set(i, { allModels: false, models: on ? [...new Set([...models, id])] : models.filter((x) => x !== id) });
        }

        const enabled = (p, id) => p.allModels || p.models.includes(id);

        // options for a kind's default: the enabled models of that kind, across providers
        function choices(kind) {
            const out = [];
            for (const p of data.providers) {
                for (const m of p.catalog) {
                    if (m.kinds.includes(kind) && enabled(p, m.id)) out.push(p.name + "/" + m.id);
                }
            }
            return out;
        }

        function field(label, input, hint) {
            return t.div({ className: "field m-b-sm" }, t.label({}, label), input, hint ? t.p({ className: "txt-hint m-t-5" }, hint) : null);
        }

        function providerCard(p, i) {
            const ti = (data.view?.types || []).find((x) => x.type === p.type) || {};
            const kinds = data.view?.kinds || [];
            const filter = p.filter.trim().toLowerCase();
            const list = p.catalog.filter((m) => !filter || m.id.toLowerCase().includes(filter) || (m.name || "").toLowerCase().includes(filter));
            const on = p.catalog.filter((m) => enabled(p, m.id)).length;
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
                p.error ? t.div({ className: "alert danger m-b-sm" }, t.div({ className: "content" }, "Listing its models failed: " + p.error)) : null,
                t.details({ open: p.open, ontoggle: (e) => (data.providers[i].open = e.target.open) },
                    t.summary({}, p.catalog.length ? `Models: ${p.allModels ? "all" : on} of ${p.catalog.length} enabled` : "Models (save to list them)"),
                    p.catalog.length ? t.div({ className: "m-t-sm" },
                        t.div({ className: "flex gap-sm m-b-sm", style: "align-items:center" },
                            t.input({ type: "search", placeholder: "Filter", value: p.filter, onchange: (e) => set(i, { filter: e.target.value }), style: "max-width:240px" }),
                            t.button({ type: "button", className: "btn transparent sm", onclick: () => set(i, { allModels: true }) }, "All"),
                            t.button({ type: "button", className: "btn transparent sm", onclick: () => set(i, { allModels: false, models: [] }) }, "None")),
                        ...kinds.map((k) => {
                            const ms = list.filter((m) => m.kinds.includes(k));
                            if (!ms.length) return null;
                            return t.div({ className: "m-b-sm" },
                                t.p({}, t.strong({}, kindLabel[k] || k), t.span({ className: "txt-hint" }, ` · ${ms.length}`)),
                                ...ms.map((m) => {
                                    const id = `llm.${i}.${k}.${m.id}`;
                                    return t.div({ className: "field", style: "margin:2px 0" },
                                        t.input({ type: "checkbox", id, checked: enabled(p, m.id), onchange: (e) => toggleModel(i, m.id, e.target.checked) }),
                                        t.label({ htmlFor: id }, m.id,
                                            m.reasoning ? t.span({ className: "label sm m-l-5" }, "reasons") : null,
                                            price(m) ? t.span({ className: "txt-hint" }, " · " + price(m)) : null));
                                }));
                        })) : null));
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
                        "The cloud models behind ", t.code({}, "$llm"), " in your hooks and pages. Add providers with your own keys, ",
                        "turn models on, and choose each kind's default. A call names a model as ", t.code({}, "provider/model"),
                        ", or none for the default."),
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
                            t.h6({ className: "m-b-sm" }, "Defaults"),
                            t.p({ className: "txt-hint m-b-sm" }, "What a call without a model uses. Save after adding a provider to list its models here."),
                            () => t.div({ className: "grid m-b-base" }, ...(data.view?.kinds || []).map((k) => {
                                const opts = choices(k);
                                const cur = data.defaults[k] || "";
                                return t.div({ className: "col-lg-6" }, field(kindLabel[k] || k,
                                    t.select({ onchange: (e) => (data.defaults = { ...data.defaults, [k]: e.target.value }) },
                                        t.option({ value: "", selected: cur === "" }, "None"),
                                        cur && !opts.includes(cur) ? t.option({ value: cur, selected: true }, cur + " (not enabled)") : null,
                                        ...opts.map((ref) => t.option({ value: ref, selected: cur === ref }, ref)))));
                            })),
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
