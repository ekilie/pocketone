// Settings → LLM for the PocketBase dashboard (PocketOne ext/llm): the providers $llm calls name as provider/model,
// each with its endpoint and key. Loaded as a UI extension (/_/extensions/llm/main.js). A view over llm's settings/v1
// section, built from the stock settings pages' pieces (accordions, fields with help under them, the save footer).
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

    const icons = { vultr: "ri-server-line", cloudflare: "ri-cloud-line", openrouter: "ri-route-line", openai: "ri-plug-line" };
    const errText = (err) => err?.response?.message || err?.message || String(err);

    // field is a stock form field: the box (label and input), with its help text under it.
    function field(label, input, hint) {
        return t.div(null, t.div({ className: "field" }, t.label({}, label), input), hint ? t.div({ className: "field-help" }, hint) : null);
    }

    app.routes.superuserOnly("#/settings/llm", () => {
        app.store.title = "LLM";
        const data = store({
            loading: true,
            saving: false,
            testing: false,
            view: null, // the section's settings/v1 values
            providers: [], // editable: { name, type, endpoint, account, key, keySet }
            saved: "", // values() as last loaded or saved, to tell when there are changes
            message: null, // { kind: "success" | "danger", text }
        });

        const typeOf = (type) => (data.view?.types || []).find((x) => x.type === type) || { type, title: type };

        function values() {
            return {
                providers: data.providers.map((p) => ({
                    name: p.name.trim(), type: p.type, endpoint: p.endpoint.trim(), account: p.account.trim(), key: p.key,
                })),
            };
        }
        const hasChanges = () => JSON.stringify(values()) !== data.saved;

        // open holds the uids of the expanded providers: kept outside the store, so editing a field (which redraws the
        // list) doesn't fold them up
        const open = new Set();
        let nextUID = 1;

        function fill(v) {
            data.view = v;
            data.providers = (v.providers || []).map((p) => ({
                uid: nextUID++, name: p.name, type: p.type, endpoint: p.endpoint || "", account: p.account || "", key: "",
                keySet: p.keySet, keyHint: p.keyHint || "",
            }));
            data.saved = JSON.stringify(values());
        }

        async function load() {
            try {
                fill((await settingsAPI("/get?extension=llm")).values);
            } catch (err) {
                app.checkApiError(err);
            }
            data.loading = false;
        }

        async function save(e) {
            e?.preventDefault();
            data.saving = true;
            data.message = null;
            try {
                fill((await settingsAPI("/save", { extension: "llm", values: values() })).values);
                app.toasts?.success ? app.toasts.success("Saved.") : (data.message = { kind: "success", text: "Saved." });
            } catch (err) {
                data.message = { kind: "danger", text: errText(err) };
            }
            data.saving = false;
        }

        async function test() {
            data.testing = true;
            data.message = null;
            try {
                const r = await settingsAPI("/test", { extension: "llm", values: values() });
                data.message = r.ok ? { kind: "success", text: "Every provider answered with its key." } : { kind: "danger", text: r.message };
            } catch (err) {
                data.message = { kind: "danger", text: errText(err) };
            }
            data.testing = false;
        }

        // set replaces one provider (so the page redraws), merging changes.
        const set = (i, changes) => (data.providers = data.providers.map((p, j) => (j === i ? { ...p, ...changes } : p)));

        // freeName is the type's suggested name, numbered when another provider has it.
        function freeName(type, except) {
            const base = typeOf(type).name || type;
            let name = base;
            for (let n = 2; data.providers.some((p, j) => j !== except && p.name === name); n++) name = base + n;
            return name;
        }

        function add() {
            const type = data.view?.types?.[0]?.type || "openai";
            const p = { uid: nextUID++, name: freeName(type), type, endpoint: "", account: "", key: "", keySet: false, keyHint: "" };
            open.add(p.uid);
            data.providers = [...data.providers, p];
            data.message = null;
        }

        // setType switches a provider's type: its endpoint and account go, and a name still at the old type's suggestion
        // follows the new one. A saved key is kept only while the type, endpoint and account stay the same (the server
        // decides), so a new type needs its key entered.
        function setType(i, type) {
            const p = data.providers[i];
            if (!type || type === p.type) return;
            const suggested = (t) => p.name === (typeOf(t).name || t) || new RegExp("^" + (typeOf(t).name || t) + "\\d+$").test(p.name);
            set(i, { type, endpoint: "", account: "", name: !p.name || suggested(p.type) ? freeName(type, i) : p.name });
        }

        function remove(i) {
            data.providers = data.providers.filter((_, j) => j !== i);
        }

        function providerAccordion(p, i) {
            const ti = typeOf(p.type);
            return t.details(
                { className: "accordion", open: open.has(p.uid), ontoggle: (e) => (e.target.open ? open.add(p.uid) : open.delete(p.uid)) },
                t.summary(
                    null,
                    t.i({ className: icons[p.type] || "ri-plug-line", ariaHidden: true }),
                    t.span({ className: "txt" }, ti.title),
                    t.span({ className: "label" }, `${p.name || "?"}/…`),
                    t.div({ className: "flex-fill" }),
                    p.key ? t.span({ className: "label success" }, "New key")
                        : p.keySet ? t.span({ className: "label success txt-mono" }, p.keyHint || "Key set")
                        : t.span({ className: "label warning" }, "No key"),
                ),
                t.div(
                    { className: "grid" },
                    t.div({ className: "col-md-5" }, field("Type",
                        app.components.select({
                            required: true,
                            options: () => (data.view?.types || []).map((x) => ({ value: x.type, label: x.title })),
                            value: () => p.type,
                            onchange: (sel) => setType(i, sel?.[0]?.value),
                        }))),
                    t.div({ className: "col-md-7" }, t.p({ className: "txt-hint m-t-sm m-b-0" }, ti.about || "")),
                    t.div({ className: "col-md-4" }, field("Name",
                        t.input({ type: "text", required: true, value: p.name,                            onchange: (e) => set(i, { name: e.target.value }) }),
                        t.span(null, "Calls name its models as ", t.code(null, `${p.name || "name"}/model`), "."))),
                    ti.account
                        ? t.div({ className: "col-md-8" }, field("Account ID",
                            t.input({ type: "text", value: p.account, autocomplete: "off", onchange: (e) => set(i, { account: e.target.value }) }),
                            "In your Cloudflare dashboard's URL, or under Account home."))
                        : t.div({ className: "col-md-8" }, field("Endpoint",
                            t.input({ type: "url", value: p.endpoint, placeholder: ti.endpoint || "https://…/v1", onchange: (e) => set(i, { endpoint: e.target.value }) }),
                            ti.endpoint ? "Leave empty for the default." : "Include the version path, like /v1.")),
                    t.div({ className: "col-lg-12" }, field(ti.account ? "API token" : "API key",
                        t.input({ type: "password", autocomplete: "new-password", value: p.key,
                            placeholder: p.keySet ? `${p.keyHint || "Saved"} (leave empty to keep it)` : "", onchange: (e) => set(i, { key: e.target.value }) }),
                        ti.account ? "A token with Workers AI permission." : null)),
                    t.div({ className: "col-lg-12 flex" },
                        t.div({ className: "m-r-auto" }),
                        t.button({ type: "button", className: "btn sm transparent danger", onclick: () => remove(i) },
                            t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }), t.span({ className: "txt" }, "Remove provider"))),
                ),
            );
        }

        load();

        return t.div(
            { className: "page page-llm-settings" },
            sidebar(),
            t.div(
                { className: "page-content full-height" },
                t.header({ className: "page-header" },
                    t.nav({ className: "breadcrumbs" },
                        t.div({ className: "breadcrumb-item" }, "Settings"),
                        t.div({ className: "breadcrumb-item" }, "LLM"))),
                t.div(
                    { className: "wrapper m-b-base" },
                    () => {
                        if (data.loading) {
                            return t.div({ className: "block txt-center" }, t.span({ className: "loader lg" }));
                        }
                        return t.form(
                            { className: "grid", onsubmit: save },
                            t.div({ className: "col-lg-12" },
                                t.p({ className: "m-t-0" },
                                    "The cloud providers behind ", t.code(null, "$llm"), " in your pages, each on your own key. ",
                                    "Every call names its model as ", t.code(null, "provider/model"), ": with a provider named ",
                                    t.code(null, "vultr"), ", ", t.code(null, "vultr/glm-5.2"), " is its glm-5.2 model."),
                            ),
                            t.div({ className: "col-lg-12" },
                                () => data.providers.length
                                    ? t.div({ className: "accordions" }, ...data.providers.map(providerAccordion))
                                    : t.div({ className: "block txt-center txt-hint p-base" }, "No providers yet."),
                            ),
                            t.div({ className: "col-lg-12" },
                                t.button({ type: "button", className: "btn secondary sm", onclick: add },
                                    t.i({ className: "ri-add-line", ariaHidden: true }), t.span({ className: "txt" }, "New provider")),
                            ),
                            () => data.message
                                ? t.div({ className: "col-lg-12" },
                                    t.div({ className: `alert ${data.message.kind}` }, t.div({ className: "content" }, data.message.text)))
                                : null,
                            t.div({ className: "col-lg-12" }, t.hr()),
                            t.div({ className: "col-lg-12" },
                                t.div({ className: "flex" },
                                    t.button({
                                        type: "button",
                                        className: () => `btn transparent secondary ${data.testing ? "loading" : ""}`,
                                        disabled: () => data.testing || !data.providers.length,
                                        onclick: test,
                                    }, t.i({ className: "ri-checkbox-circle-line", ariaHidden: true }), t.span({ className: "txt" }, "Test keys")),
                                    t.div({ className: "m-r-auto" }),
                                    t.button({
                                        type: "button",
                                        className: "btn transparent secondary",
                                        hidden: () => !hasChanges(),
                                        disabled: () => data.saving,
                                        onclick: () => { data.message = null; load(); },
                                    }, t.span({ className: "txt" }, "Cancel")),
                                    t.button({
                                        className: () => `btn expanded-lg ${data.saving ? "loading" : ""}`,
                                        disabled: () => data.saving || !hasChanges(),
                                    }, t.span({ className: "txt" }, "Save changes")),
                                ),
                            ),
                        );
                    },
                ),
                t.footer({ className: "page-footer" }, app.components.credits()),
            ),
        );
    });
})();
