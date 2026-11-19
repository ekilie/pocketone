// Settings → LLM for the PocketBase dashboard (PocketOne ext/agent): what answers $agent.complete. Loaded as a UI
// extension (/_/extensions/agent/main.js). A view over agent's settings/v1 section and agent/v1's status.
//
// On a single app: Off, Built-in (a model PocketOne runs) or API (your own OpenAI-compatible endpoint). Under a
// multisite host the host decides which model runs built in, so the choices are Off, the host's model or API.
// Until the admins choose, pocketone.yaml's agent section decides (and under a host, the host's model answers).
(() => {
    const SYSTEM = "System";
    app.store.settingsNavGroups[SYSTEM] = [
        ...(app.store.settingsNavGroups[SYSTEM] || []),
        { href: "#/settings/llm", icon: "ri-sparkling-2-line", label: "LLM" },
    ];

    const settingsAPI = (path, body) =>
        app.pb.send("/api/_c/settings/v1" + path, body ? { method: "POST", body, requestKey: null } : { requestKey: null });
    const statusAPI = () => app.pb.send("/api/_c/agent/v1", { requestKey: null });

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

    const size = (b) => (b ? (b >= 1e9 ? `${(b / 1e9).toFixed(1)} GB` : `${Math.round(b / 1e6)} MB`) : "");
    const modeLabel = { off: "Off", builtin: "Built-in", host: "Your host's model", api: "API" };

    app.routes.superuserOnly("#/settings/llm", () => {
        app.store.title = "LLM";
        const data = store({
            loading: true,
            saving: false,
            view: null, // the section's settings/v1 values
            status: null, // agent/v1 status
            form: { mode: "", model: "", endpoint: "", key: "", apiModel: "" },
            message: null, // { kind: "success" | "danger", text }
        });
        let poll = null;

        async function loadStatus() {
            try {
                data.status = await statusAPI();
            } catch (_) {
                data.status = null;
            }
            clearTimeout(poll);
            // a model downloading or loading: keep the progress fresh while the page is open
            if (data.status && data.status.backend && !data.status.ready && location.hash.startsWith("#/settings/llm")) {
                poll = setTimeout(loadStatus, 1500);
            }
        }

        function fill(v) {
            data.view = v;
            const mode = v.mode || v.effective;
            data.form = {
                mode,
                model: v.model || (v.effective === "builtin" ? v.effectiveModel : "") || v.models?.[0]?.name || "",
                endpoint: v.endpoint || "",
                key: "",
                apiModel: v.apiModel || "",
            };
        }

        async function load() {
            try {
                fill((await settingsAPI("/get?extension=agent")).values);
            } catch (err) {
                app.checkApiError(err);
            }
            data.loading = false;
            loadStatus();
        }

        function values() {
            const f = data.form;
            if (f.mode === "builtin") return { mode: f.mode, model: f.model };
            if (f.mode === "api") return { mode: f.mode, endpoint: f.endpoint, key: f.key, apiModel: f.apiModel };
            return { mode: f.mode };
        }

        async function save(e, override) {
            e?.preventDefault();
            data.saving = true;
            data.message = null;
            try {
                fill((await settingsAPI("/save", { extension: "agent", values: override || values() })).values);
                data.message = { kind: "success", text: "Saved." };
                loadStatus();
            } catch (err) {
                data.message = { kind: "danger", text: err?.response?.message || err?.message || String(err) };
            }
            data.saving = false;
        }

        async function test() {
            data.message = null;
            try {
                const r = await settingsAPI("/test", { extension: "agent", values: values() });
                data.message = r.ok ? { kind: "success", text: "The API answered." } : { kind: "danger", text: r.message };
            } catch (err) {
                data.message = { kind: "danger", text: err?.response?.message || err?.message || String(err) };
            }
        }

        function current() {
            const v = data.view;
            if (!v) return null;
            const what = v.effective === "off" ? "Off"
                : v.effective === "api" ? `Your API${v.apiModel ? ` (${v.apiModel})` : ""}`
                : `${modeLabel[v.effective]}: ${v.effectiveModel || "no model named"}`;
            const why = v.source === "pocketone.yaml" ? "from pocketone.yaml (agent.model), until you choose below"
                : v.source === "host" ? "your host's default, until you choose below"
                : v.source === "none" ? "nothing is configured" : "chosen here";
            return t.p({}, t.strong({}, "In use: "), what, t.span({ className: "txt-hint" }, ` · ${why}`),
                v.source === "settings" && (v.yamlModel || v.hosted)
                    ? t.button({ type: "button", className: "btn transparent sm m-l-5", onclick: () => save(null, { mode: "" }) },
                        v.hosted ? "Reset to the host's default" : "Reset to pocketone.yaml")
                    : null);
        }

        function statusLine() {
            const s = data.status;
            if (!s || !s.backend) return null;
            if (s.ready) {
                return t.p({ className: "txt-hint" }, t.i({ className: "ri-checkbox-circle-line txt-success" }), ` Ready${s.model ? `: ${s.model}` : ""}`);
            }
            return t.div({ className: "m-b-sm" },
                t.p({ className: "txt-hint" }, t.span({ className: "loader sm" }), " ", s.message || "Loading the model…"),
                s.progress ? t.progress({ value: s.progress, max: 1, style: "width:100%" }) : null);
        }

        function choice(mode, label, hint) {
            const id = "llm.mode." + mode;
            return t.div({ className: "field m-b-sm" },
                t.input({ type: "radio", id, name: "llm.mode", checked: () => data.form.mode === mode, onchange: () => (data.form.mode = mode) }),
                t.label({ htmlFor: id }, t.strong({}, label), t.span({ className: "txt-hint" }, " · " + hint)));
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
                    t.p({ className: "txt-hint" },
                        "The language model behind ", t.code({}, "$agent.complete"), " and ", t.code({}, "$agent.stream"),
                        " in your hooks and pages."),
                    () => data.loading
                        ? t.div({ className: "block txt-center" }, t.span({ className: "loader lg" }))
                        : t.form({ onsubmit: save },
                            current,
                            statusLine,
                            choice("off", "Off", "$agent answers with an error"),
                            () => data.view?.hosted
                                ? choice("host", "Your host's model", data.view.yamlModel
                                    ? `${data.view.yamlModel}, run by your host and shared with its other apps`
                                    : "your host runs no model right now")
                                : choice("builtin", "Built-in", "a model PocketOne downloads once and runs on this server's CPU"),
                            () => data.form.mode === "builtin" && !data.view?.hosted
                                ? t.div({ className: "field m-b-base", style: "margin-left:28px; width:calc(100% - 28px)" },
                                    t.label({ htmlFor: "llm.model" }, "Model"),
                                    t.select({ id: "llm.model", onchange: (e) => (data.form.model = e.target.value) },
                                        ...(data.view?.models || []).map((m) => t.option(
                                            { value: m.name, selected: () => data.form.model === m.name },
                                            `${m.name}${m.size ? ` (${size(m.size)})` : ""} · ${m.about}`))),
                                    t.p({ className: "txt-hint m-t-5" }, "The first use downloads it; the status above shows the progress."))
                                : null,
                            choice("api", "API", "your own OpenAI-compatible endpoint: OpenAI, OpenRouter, Groq, Ollama…"),
                            () => data.form.mode === "api"
                                ? t.div({ className: "grid m-b-base", style: "margin-left:28px; width:calc(100% - 28px)" },
                                    t.div({ className: "col-lg-12" }, t.div({ className: "field" },
                                        t.label({ htmlFor: "llm.endpoint" }, "Endpoint"),
                                        t.input({ id: "llm.endpoint", type: "url", required: true, placeholder: "https://api.openai.com/v1",
                                            value: () => data.form.endpoint, oninput: (e) => (data.form.endpoint = e.target.value) }))),
                                    t.div({ className: "col-lg-6" }, t.div({ className: "field" },
                                        t.label({ htmlFor: "llm.key" }, "Key"),
                                        t.input({ id: "llm.key", type: "password", autocomplete: "off",
                                            placeholder: () => data.view?.keySet && data.form.endpoint === data.view?.endpoint ? "saved (leave empty to keep it)" : "",
                                            value: () => data.form.key, oninput: (e) => (data.form.key = e.target.value) }))),
                                    t.div({ className: "col-lg-6" }, t.div({ className: "field" },
                                        t.label({ htmlFor: "llm.apiModel" }, "Model"),
                                        t.input({ id: "llm.apiModel", type: "text", placeholder: "gpt-4o-mini",
                                            value: () => data.form.apiModel, oninput: (e) => (data.form.apiModel = e.target.value) }))),
                                )
                                : null,
                            () => data.message
                                ? t.div({ className: `alert ${data.message.kind} m-b-sm` }, t.div({ className: "content" }, data.message.text))
                                : null,
                            t.div({ className: "flex gap-sm" },
                                t.button({ type: "submit", className: "btn", disabled: () => data.saving }, "Save"),
                                () => data.form.mode === "api"
                                    ? t.button({ type: "button", className: "btn secondary", onclick: test }, "Test")
                                    : null),
                        ),
                ),
                t.footer({ className: "page-footer" }, app.components.credits()),
            ),
        );
    });
})();
