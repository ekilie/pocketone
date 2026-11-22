// The Search screen for the PocketBase dashboard, for the search/v1 contract (PocketOne ext/contracts/search/v1): every
// search bucket of the app, its documents, a drop zone and a paste box to add content, and a search to try. It calls
// only search/v1 (and llm.embed/v1's model list, when something provides it), so it works whoever provides the
// contract. Served at /_/extensions/search/main.js.
(() => {
    const api = "/api/_c/search/v1";
    const send = (path, body) =>
        app.pb.send(api + path, body !== undefined ? { method: "POST", body, requestKey: null } : { requestKey: null });
    const errText = (err) => err?.response?.message || err?.message || String(err);
    const MAX_FILE = 10 << 20;

    app.store.headerLinks = [
        ...app.store.headerLinks.filter((l) => l.href !== "#/settings"),
        { href: "#/search", icon: "ri-search-eye-line", label: "Search" },
        ...app.store.headerLinks.filter((l) => l.href === "#/settings"),
    ];

    const style = document.createElement("style");
    style.textContent = `
        .search-dropzone { border: 2px dashed var(--borderColor); border-radius: var(--baseRadius); padding: var(--smSpacing);
            text-align: center; color: var(--txtHintColor); transition: border-color 0.15s, background 0.15s; }
        .search-dropzone.over { border-color: var(--primaryColor); background: var(--baseAlt1Color); }
        .search-table { width: 100%; border-collapse: collapse; }
        .search-table th, .search-table td { text-align: left; padding: 6px 10px; border-bottom: 1px solid var(--baseAlt2Color); vertical-align: top; }
        .search-table th { font-weight: 600; color: var(--txtHintColor); font-size: var(--smFontSize); }
        .search-table tr.link { cursor: pointer; } .search-table tr.link:hover td { background: var(--baseAlt1Color); }
        .search-result { border: 1px solid var(--baseAlt2Color); border-radius: var(--baseRadius); padding: 10px; margin-bottom: 8px; }
        .search-result pre { white-space: pre-wrap; margin: 6px 0 0; max-height: 240px; overflow: auto; font-size: var(--smFontSize); }
        .search-num { text-align: right !important; font-variant-numeric: tabular-nums; }`;
    document.head.append(style);

    const fmtBytes = (n) => (n >= 1 << 20 ? (n / (1 << 20)).toFixed(1) + " MB" : n >= 1024 ? Math.round(n / 1024) + " KB" : n + " B");
    const fmtCost = (micro) => (micro ? "$" + (micro / 1e6).toFixed(micro < 10000 ? 4 : 2) : "");
    const statusLabel = (s, err) => t.span({
        className: `label ${s === "ready" ? "success" : s === "error" ? "danger" : "warning"}`,
        title: err || null,
    }, s);

    async function embedModels() {
        try {
            const r = await app.pb.send("/api/_c/llm.embed/v1/models", { requestKey: null });
            return (r.models || []).map((m) => m.provider + "/" + m.id);
        } catch (_) {
            return []; // no llm.embed/v1 here, or no providers: keyword search only
        }
    }

    // modelField edits obj[key], suggesting the models in models() (llm.embed/v1's list)
    function modelField(obj, key, models) {
        return t.div({ className: "field" },
            t.label({}, "Embedding model"),
            t.input({
                type: "text", "html-list": "search-models", placeholder: "provider/model, or empty for keyword search only",
                value: () => obj[key], oninput: (e) => (obj[key] = e.target.value),
            }),
            t.datalist({ id: "search-models" }, () => models().map((m) => t.option({ value: m }))));
    }

    function page(title, crumbs, body) {
        app.store.title = title;
        return t.div({ className: "page page-search" },
            t.div({ className: "page-content full-height" },
                t.header({ className: "page-header" }, t.nav({ className: "breadcrumbs" }, ...crumbs)),
                t.div({ className: "wrapper m-b-base" }, body),
                t.footer({ className: "page-footer" }, app.components.credits())));
    }

    // ---- the bucket list

    app.routes.superuserOnly("#/search", () => {
        const data = store({ loading: true, buckets: [], models: [], creating: false, name: "", title: "", model: "", dims: "" });

        async function load() {
            try {
                data.buckets = (await send("/buckets")).buckets || [];
            } catch (err) {
                app.checkApiError(err);
            }
            data.loading = false;
        }

        async function create(e) {
            e.preventDefault();
            try {
                const b = await send("/ensure", {
                    name: data.name.trim(), title: data.title.trim(), model: data.model.trim(), dimensions: parseInt(data.dims) || 0,
                });
                window.location.hash = "#/search/" + encodeURIComponent(b.name);
            } catch (err) {
                app.toasts.error(errText(err));
            }
        }

        load();
        embedModels().then((m) => (data.models = m));

        return page("Search", [t.div({ className: "breadcrumb-item" }, "Search")], () => {
            if (data.loading) return t.div({ className: "block txt-center" }, t.span({ className: "loader lg" }));
            return t.div(null,
                t.p({ className: "m-t-0" },
                    "Buckets of content your app searches with ", t.code(null, "$search"), ". An app makes the bucket it needs; ",
                    "load its content here by dropping files on it or pasting text."),
                data.buckets.length
                    ? t.table({ className: "search-table m-b-base" },
                        t.thead(null, t.tr(null, t.th(null, "Bucket"), t.th(null, "Made by"), t.th({ className: "search-num" }, "Documents"),
                            t.th({ className: "search-num" }, "Chunks"), t.th({ className: "search-num" }, "Size"), t.th(null, "Model"), t.th(null, "Status"))),
                        t.tbody(null, ...data.buckets.map((b) => t.tr({ className: "link", onclick: () => (window.location.hash = "#/search/" + encodeURIComponent(b.name)) },
                            t.td(null, t.strong(null, b.title || b.name), b.title ? t.div({ className: "txt-hint txt-sm" }, b.name) : null),
                            t.td(null, b.owner || ""),
                            t.td({ className: "search-num" }, b.documents),
                            t.td({ className: "search-num" }, b.chunks),
                            t.td({ className: "search-num" }, fmtBytes(b.bytes)),
                            t.td(null, b.model ? t.code(null, b.model) : t.span({ className: "txt-hint" }, "keywords only")),
                            t.td(null, b.documents === 0 ? t.span({ className: "label warning" }, "empty")
                                : b.errors ? t.span({ className: "label danger" }, b.errors + " failed")
                                    : b.pending ? t.span({ className: "label warning" }, b.pending + " embedding")
                                        : t.span({ className: "label success" }, "ready"))))))
                    : t.div({ className: "block txt-center txt-hint p-base" }, "No buckets yet."),
                () => data.creating
                    ? t.form({ className: "grid", onsubmit: create },
                        t.div({ className: "col-lg-4" }, t.div({ className: "field" }, t.label({}, "Name"),
                            t.input({ type: "text", required: true, pattern: "[A-Za-z0-9][A-Za-z0-9_\\-]{0,63}", placeholder: "docs",
                                value: () => data.name, oninput: (e) => (data.name = e.target.value) }))),
                        t.div({ className: "col-lg-8" }, t.div({ className: "field" }, t.label({}, "Title"),
                            t.input({ type: "text", value: () => data.title, oninput: (e) => (data.title = e.target.value) }))),
                        t.div({ className: "col-lg-8" }, modelField(data, "model", () => data.models)),
                        t.div({ className: "col-lg-4" }, t.div({ className: "field" }, t.label({}, "Dimensions"),
                            t.input({ type: "number", min: 0, placeholder: "the model's own", value: () => data.dims, oninput: (e) => (data.dims = e.target.value) }))),
                        t.div({ className: "col-lg-12 flex" },
                            t.button({ type: "button", className: "btn transparent secondary", onclick: () => (data.creating = false) }, t.span({ className: "txt" }, "Cancel")),
                            t.button({ className: "btn" }, t.span({ className: "txt" }, "Create bucket"))))
                    : t.button({ type: "button", className: "btn secondary sm", onclick: () => (data.creating = true) },
                        t.i({ className: "ri-add-line", ariaHidden: true }), t.span({ className: "txt" }, "New bucket")),
            );
        });
    });

    // ---- one bucket

    // files under a dropped item, with their paths (folders are walked)
    async function filesOf(dataTransfer) {
        const out = [];
        const walk = (entry, prefix) => new Promise((resolve) => {
            if (entry.isFile) {
                entry.file((f) => { out.push({ file: f, name: prefix + f.name }); resolve(); }, () => resolve());
            } else if (entry.isDirectory) {
                const reader = entry.createReader();
                const all = [];
                const next = () => reader.readEntries(async (batch) => {
                    if (!batch.length) {
                        for (const e of all) await walk(e, prefix + entry.name + "/");
                        resolve();
                    } else {
                        all.push(...batch);
                        next();
                    }
                }, () => resolve());
                next();
            } else resolve();
        });
        const items = [...(dataTransfer.items || [])].map((i) => i.webkitGetAsEntry?.()).filter(Boolean);
        if (items.length) {
            for (const e of items) await walk(e, "");
        } else {
            for (const f of dataTransfer.files || []) out.push({ file: f, name: f.name });
        }
        return out;
    }

    app.routes.superuserOnly("#/search/{name}", (route) => {
        const name = decodeURIComponent(route.params.name);
        const data = store({
            loading: true, bucket: null, documents: [], models: [], over: false, adding: "",
            pasteOpen: false, pasteName: "", pasteText: "",
            query: "", mode: "", searching: false, results: null,
            settingsOpen: false, edit: {},
        });
        let timer = null;

        async function load() {
            try {
                const [b, d] = await Promise.all([send("/bucket?name=" + encodeURIComponent(name)), send("/documents?bucket=" + encodeURIComponent(name))]);
                data.bucket = b;
                data.documents = d.documents || [];
            } catch (err) {
                app.checkApiError(err);
            }
            data.loading = false;
            clearTimeout(timer);
            if (data.bucket?.pending && document.querySelector(".page-search-bucket")) timer = setTimeout(load, 2000);
        }

        async function addFiles(list) {
            let ok = 0, unchanged = 0;
            const failed = [];
            for (const [i, { file, name: docName }] of list.entries()) {
                data.adding = `Adding ${i + 1} of ${list.length}: ${docName}`;
                if (file.size > MAX_FILE) {
                    failed.push(`${docName}: over 10 MB`);
                    continue;
                }
                try {
                    const d = await send("/add", { bucket: name, name: docName, text: await file.text() });
                    d.unchanged ? unchanged++ : ok++;
                } catch (err) {
                    failed.push(`${docName}: ${errText(err)}`);
                }
            }
            data.adding = "";
            if (ok || unchanged) app.toasts.success(`Added ${ok} document${ok === 1 ? "" : "s"}` + (unchanged ? `, ${unchanged} unchanged` : "") + ".");
            if (failed.length) app.toasts.error(`${failed.length} not added: ${failed.slice(0, 3).join("; ")}${failed.length > 3 ? "; …" : ""}`);
            load();
        }

        async function paste(e) {
            e.preventDefault();
            try {
                await send("/add", { bucket: name, name: data.pasteName.trim(), text: data.pasteText });
                data.pasteOpen = false;
                data.pasteName = data.pasteText = "";
                load();
            } catch (err) {
                app.toasts.error(errText(err));
            }
        }

        function remove(doc) {
            app.modals.confirm(`Remove ${doc.name} from the bucket?`, async () => {
                try {
                    await send("/remove", { bucket: name, names: [doc.name] });
                } catch (err) {
                    app.toasts.error(errText(err));
                }
                load();
            });
        }

        async function reindex(names) {
            try {
                await send("/reindex", { bucket: name, names });
            } catch (err) {
                app.toasts.error(errText(err));
            }
            load();
        }

        async function search(e) {
            e.preventDefault();
            if (!data.query.trim()) return;
            data.searching = true;
            try {
                data.results = await send("/search", { bucket: name, query: data.query, mode: data.mode, limit: 8 });
            } catch (err) {
                data.results = null;
                app.toasts.error(errText(err));
            }
            data.searching = false;
        }

        function openSettings() {
            const b = data.bucket;
            data.edit = { title: b.title, description: b.description, model: b.model, dims: b.dimensions || "", maxDocuments: b.maxDocuments, maxMB: Math.round(b.maxBytes / (1 << 20)) };
            data.settingsOpen = true;
            embedModels().then((m) => (data.models = m));
        }

        async function saveSettings(e) {
            e.preventDefault();
            const b = data.bucket, x = data.edit;
            const body = { name, title: x.title, description: x.description, maxDocuments: parseInt(x.maxDocuments) || 0, maxBytes: (parseInt(x.maxMB) || 0) * (1 << 20) };
            const dims = parseInt(x.dims) || 0;
            if (x.model.trim() !== b.model || dims !== b.dimensions) {
                const est = b.chunks ? ` All ${b.chunks} chunks (about ${Math.round(data.documents.reduce((n, d) => n + d.tokens, 0) / 1000)}K tokens) are embedded again.` : "";
                if (!confirm(`Change the embedding model?${est}`)) return;
                body.model = x.model.trim();
                body.dimensions = dims;
            }
            try {
                await send("/update", body);
                data.settingsOpen = false;
                load();
            } catch (err) {
                app.toasts.error(errText(err));
            }
        }

        function deleteBucket() {
            app.modals.confirm(`Delete the bucket ${name} and everything in it? Apps that search it will find nothing until it's loaded again.`, async () => {
                try {
                    await send("/delete", { name });
                    window.location.hash = "#/search";
                } catch (err) {
                    app.toasts.error(errText(err));
                }
            });
        }

        load();

        const crumbs = [
            t.a({ className: "breadcrumb-item", href: "#/search" }, "Search"),
            t.div({ className: "breadcrumb-item" }, () => data.bucket?.title || name),
        ];

        return t.div({ className: "page-search-bucket", onunmount: () => clearTimeout(timer) }, page(name, crumbs, () => {
            if (data.loading) return t.div({ className: "block txt-center" }, t.span({ className: "loader lg" }));
            const b = data.bucket;
            if (!b) return t.div({ className: "block txt-center txt-hint p-base" }, "No bucket named ", t.code(null, name), ".");
            return t.div(null,
                b.description ? t.div({ className: "alert info m-b-sm" }, t.div({ className: "content" },
                    b.owner ? t.strong(null, b.owner + " wants: ") : null, b.description)) : null,
                t.p({ className: "txt-hint m-t-0" },
                    `${b.documents} document${b.documents === 1 ? "" : "s"}, ${b.chunks} chunks, ${fmtBytes(b.bytes)}. `,
                    b.model ? ["Embedded with ", t.code(null, b.model), b.dimensions ? ` (${b.dimensions} dimensions)` : "", "."] : "Searched by keywords only (no embedding model).",
                    " ", t.a({ href: "#", onclick: (e) => { e.preventDefault(); openSettings(); } }, "Settings")),

                () => data.settingsOpen ? t.form({ className: "grid m-b-base", onsubmit: saveSettings },
                    t.div({ className: "col-lg-6" }, t.div({ className: "field" }, t.label({}, "Title"),
                        t.input({ type: "text", value: () => data.edit.title, oninput: (e) => (data.edit.title = e.target.value) }))),
                    t.div({ className: "col-lg-6" }, modelField(data.edit, "model", () => data.models)),
                    t.div({ className: "col-lg-12" }, t.div({ className: "field" }, t.label({}, "Description (what belongs in the bucket)"),
                        t.textarea({ rows: 2, value: () => data.edit.description, oninput: (e) => (data.edit.description = e.target.value) }))),
                    t.div({ className: "col-lg-4" }, t.div({ className: "field" }, t.label({}, "Dimensions"),
                        t.input({ type: "number", min: 0, placeholder: "the model's own", value: () => data.edit.dims, oninput: (e) => (data.edit.dims = e.target.value) }))),
                    t.div({ className: "col-lg-4" }, t.div({ className: "field" }, t.label({}, "Most documents"),
                        t.input({ type: "number", min: 0, value: () => data.edit.maxDocuments, oninput: (e) => (data.edit.maxDocuments = e.target.value) }))),
                    t.div({ className: "col-lg-4" }, t.div({ className: "field" }, t.label({}, "Most text (MB)"),
                        t.input({ type: "number", min: 0, value: () => data.edit.maxMB, oninput: (e) => (data.edit.maxMB = e.target.value) }))),
                    t.div({ className: "col-lg-12 flex" },
                        t.button({ type: "button", className: "btn sm transparent danger", onclick: deleteBucket },
                            t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }), t.span({ className: "txt" }, "Delete bucket")),
                        t.div({ className: "m-r-auto" }),
                        t.button({ type: "button", className: "btn transparent secondary", onclick: () => (data.settingsOpen = false) }, t.span({ className: "txt" }, "Cancel")),
                        t.button({ className: "btn" }, t.span({ className: "txt" }, "Save")))) : null,

                t.div({
                    className: () => `search-dropzone m-b-sm ${data.over ? "over" : ""}`,
                    ondragover: (e) => { e.preventDefault(); data.over = true; },
                    ondragleave: () => (data.over = false),
                    ondrop: async (e) => {
                        e.preventDefault();
                        data.over = false;
                        const list = await filesOf(e.dataTransfer);
                        if (list.length) addFiles(list);
                    },
                },
                    () => data.adding ? t.span(null, t.span({ className: "loader sm m-r-5" }), data.adding) : t.span(null,
                        t.i({ className: "ri-upload-cloud-line", ariaHidden: true }),
                        " Drop files or folders here (Markdown, text, code), ",
                        t.label({ className: "link-primary", style: "cursor: pointer" }, "choose files",
                            t.input({ type: "file", multiple: true, hidden: true, onchange: (e) => addFiles([...e.target.files].map((f) => ({ file: f, name: f.name }))) })),
                        ", or ",
                        t.a({ href: "#", className: "link-primary", onclick: (e) => { e.preventDefault(); data.pasteOpen = !data.pasteOpen; } }, "paste text"),
                        ".")),

                () => data.pasteOpen ? t.form({ className: "grid m-b-base", onsubmit: paste },
                    t.div({ className: "col-lg-12" }, t.div({ className: "field" }, t.label({}, "Name"),
                        t.input({ type: "text", required: true, placeholder: "notes/characters.md", value: () => data.pasteName, oninput: (e) => (data.pasteName = e.target.value) }))),
                    t.div({ className: "col-lg-12" }, t.div({ className: "field" }, t.label({}, "Text"),
                        t.textarea({ rows: 8, required: true, value: () => data.pasteText, oninput: (e) => (data.pasteText = e.target.value) }))),
                    t.div({ className: "col-lg-12 flex" }, t.div({ className: "m-r-auto" }),
                        t.button({ className: "btn" }, t.span({ className: "txt" }, "Add")))) : null,

                data.documents.length
                    ? t.table({ className: "search-table m-b-base" },
                        t.thead(null, t.tr(null, t.th(null, "Document"), t.th(null, "Kind"), t.th({ className: "search-num" }, "Size"),
                            t.th({ className: "search-num" }, "Chunks"), t.th({ className: "search-num" }, "Cost"), t.th(null, "Status"), t.th(null, ""))),
                        t.tbody(null, ...data.documents.map((d) => t.tr(null,
                            t.td(null, d.name, d.error ? t.div({ className: "txt-danger txt-sm" }, d.error) : null),
                            t.td(null, d.kind),
                            t.td({ className: "search-num" }, fmtBytes(d.size)),
                            t.td({ className: "search-num" }, d.chunks),
                            t.td({ className: "search-num" }, fmtCost(d.costMicroUSD)),
                            t.td(null, statusLabel(d.status, d.error)),
                            t.td({ className: "txt-right" },
                                d.status === "error" ? t.button({ type: "button", className: "btn xs transparent secondary", onclick: () => reindex([d.name]) }, "Retry") : null,
                                t.button({ type: "button", className: "btn xs transparent danger", title: "Remove", onclick: () => remove(d) },
                                    t.i({ className: "ri-delete-bin-7-line", ariaHidden: true })))))))
                    : t.div({ className: "block txt-center txt-hint p-base" }, "Nothing in this bucket yet."),

                t.hr(),
                t.div({ className: "txt-lg m-b-sm" }, "Try a search"),
                t.form({ className: "flex gap-sm m-b-sm", onsubmit: search },
                    t.input({ type: "text", placeholder: "A question or a few words", style: "flex: 1",
                        value: () => data.query, oninput: (e) => (data.query = e.target.value) }),
                    t.select({ onchange: (e) => (data.mode = e.target.value) },
                        t.option({ value: "" }, "Meaning + keywords"), t.option({ value: "semantic" }, "Meaning"), t.option({ value: "keyword" }, "Keywords")),
                    t.button({ className: () => `btn ${data.searching ? "loading" : ""}` }, t.span({ className: "txt" }, "Search"))),
                () => {
                    const r = data.results;
                    if (!r) return null;
                    if (!r.results.length) return t.div({ className: "txt-hint" }, "No matches.");
                    return t.div(null,
                        t.div({ className: "txt-hint txt-sm m-b-sm" }, r.note ? r.note : `Ranked by ${r.mode}` + (r.usage?.costMicroUSD ? `; the query cost ${fmtCost(r.usage.costMicroUSD)}` : "") + "."),
                        ...r.results.map((x) => t.div({ className: "search-result" },
                            t.div({ className: "flex gap-sm" },
                                t.strong(null, x.document), x.heading ? t.span({ className: "txt-hint" }, "› " + x.heading) : null,
                                t.span({ className: "txt-hint txt-sm" }, `lines ${x.startLine}–${x.endLine}`),
                                t.span({ className: "m-l-auto label" }, x.score.toFixed(4)),
                                x.semantic ? t.span({ className: "label info", title: "cosine similarity" }, "meaning " + x.semantic.toFixed(3)) : null,
                                x.keyword ? t.span({ className: "label", title: "BM25" }, "keywords " + x.keyword.toFixed(2)) : null),
                            t.pre(null, x.text))));
                },
            );
        }));
    });
})();
