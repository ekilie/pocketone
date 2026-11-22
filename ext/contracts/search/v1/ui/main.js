// The Search screen for the PocketBase dashboard, for the search/v1 contract (PocketOne ext/contracts/search/v1): every
// search bucket of the app, its documents, a drop zone and a paste box to add content, a collection a bucket can
// follow, a search to try, and the app's embedding model. It calls only search/v1 (and llm.embed/v1's model list for
// the model picker), so it works whoever provides the contract. Served at /_/extensions/search/main.js.
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
    }, s === "queued" ? "waiting" : s);
    const recordHref = (r) => `#/collections?collection=${encodeURIComponent(r.collection)}&recordId=${encodeURIComponent(r.id)}`;

    // embedModels is llm.embed/v1's list: [{ id: "provider/model", name, prices }]
    async function embedModels() {
        try {
            return (await app.pb.send("/api/_c/llm.embed/v1/models", { requestKey: null })).models || [];
        } catch (_) {
            return [];
        }
    }
    const modelLabel = (m) => {
        const p = (m.prices || []).find((x) => /token/i.test(x.unit));
        return (m.name || m.id) + (p ? ` ($${p.usd} per ${p.unit})` : "");
    };

    function page(title, crumbs, body) {
        app.store.title = title;
        return t.div({ className: "page page-search" },
            t.div({ className: "page-content full-height" },
                t.header({ className: "page-header" }, t.nav({ className: "breadcrumbs" }, ...crumbs)),
                t.div({ className: "wrapper m-b-base" }, body),
                t.footer({ className: "page-footer" }, app.components.credits())));
    }

    // results draws a search's matches
    function results(r) {
        if (!r) return null;
        if (!r.results.length) return t.div({ className: "txt-hint" }, "No matches.");
        return t.div(null,
            t.div({ className: "txt-hint txt-sm m-b-sm" }, `Ranked by meaning and keywords with ${r.model}` +
                (r.usage?.costMicroUSD ? `; the query cost ${fmtCost(r.usage.costMicroUSD)}` : "") + "."),
            ...r.results.map((x) => t.div({ className: "search-result" },
                t.div({ className: "flex gap-sm" },
                    t.span({ className: "label" }, x.bucket),
                    x.record ? t.a({ href: recordHref(x.record) }, t.strong(null, x.document)) : t.strong(null, x.document),
                    x.heading ? t.span({ className: "txt-hint" }, "› " + x.heading) : null,
                    t.span({ className: "txt-hint txt-sm" }, `lines ${x.startLine}–${x.endLine}`),
                    t.span({ className: "m-l-auto txt-hint txt-sm" }, x.score.toFixed(4))),
                t.pre(null, x.text))));
    }

    // searchBox is a query box over the given buckets (none: every bucket)
    function searchBox(buckets) {
        const s = store({ query: "", searching: false, results: null });
        async function run(e) {
            e.preventDefault();
            if (!s.query.trim()) return;
            s.searching = true;
            try {
                s.results = await send("/search", { buckets, query: s.query, limit: 8 });
            } catch (err) {
                s.results = null;
                app.toasts.error(errText(err));
            }
            s.searching = false;
        }
        return t.div(null,
            t.form({ className: "flex gap-sm m-b-sm", onsubmit: run },
                t.input({ type: "text", placeholder: "A question or a few words", style: "flex: 1",
                    value: () => s.query, oninput: (e) => (s.query = e.target.value) }),
                t.button({ className: () => `btn ${s.searching ? "loading" : ""}` }, t.span({ className: "txt" }, "Search"))),
            () => results(s.results));
    }

    // ---- the bucket list

    app.routes.superuserOnly("#/search", () => {
        const data = store({
            loading: true, buckets: [], status: null, models: [], changingModel: false, model: "",
            creating: false, name: "", title: "", description: "",
        });

        let timer = null;
        async function load() {
            try {
                const [b, st] = await Promise.all([send("/buckets"), send("/status")]);
                data.buckets = b.buckets || [];
                data.status = st;
            } catch (err) {
                app.checkApiError(err);
            }
            data.loading = false;
            // while embedding runs (a new model, or content just added), follow it
            clearTimeout(timer);
            const st = data.status;
            if (st?.model && st.embedded < st.chunks && document.querySelector(".page-search-list")) timer = setTimeout(load, 2000);
        }

        async function create(e) {
            e.preventDefault();
            try {
                const b = await send("/ensure", { name: data.name.trim(), title: data.title.trim(), description: data.description.trim() });
                window.location.hash = "#/search/" + encodeURIComponent(b.name);
            } catch (err) {
                app.toasts.error(errText(err));
            }
        }

        async function openModel() {
            data.models = await embedModels();
            data.model = data.status?.model || "";
            data.changingModel = true;
        }

        async function saveModel(e) {
            e.preventDefault();
            if (data.model === data.status?.model) return (data.changingModel = false);
            const chunks = data.status?.chunks || 0;
            if (chunks && !confirm(`Embed all ${chunks} chunks again with the new model? Search keeps working (by keywords) while it runs.`)) return;
            try {
                data.status = await send("/set-model", { model: data.model });
                data.changingModel = false;
            } catch (err) {
                app.toasts.error(errText(err));
            }
        }

        load();

        // progress is how far embedding has got across every bucket
        function progress(st) {
            const pct = st.chunks ? Math.floor((100 * st.embedded) / st.chunks) : 0;
            return t.div({ className: "alert info m-t-sm" }, t.div({ className: "content" },
                t.div({ className: "flex gap-sm" },
                    t.span({ className: "loader sm" }),
                    t.span(null, `Embedding: ${st.embedded} of ${st.chunks} chunks (${pct}%). Search keeps working; chunks not done yet are found by keywords.`)),
                t.div({ style: "height: 6px; border-radius: 3px; background: var(--baseAlt2Color); margin-top: 8px; overflow: hidden" },
                    t.div({ style: `height: 100%; width: ${pct}%; background: var(--primaryColor); transition: width 0.5s` }))));
        }

        function modelCard() {
            const st = data.status;
            if (!st) return null;
            if (!st.model) {
                return t.div({ className: "alert warning m-b-sm" }, t.div({ className: "content" },
                    "Searching needs an AI provider with embedding models. Add one in ",
                    t.a({ href: "#/settings/llm" }, "Settings → LLM"),
                    "; content you add meanwhile waits and is embedded once there is one."));
            }
            return t.div({ className: "m-b-sm" },
                () => data.changingModel
                    ? t.form({ className: "flex gap-sm", onsubmit: saveModel },
                        t.select({ onchange: (e) => (data.model = e.target.value) },
                            ...data.models.map((m) => t.option({ value: m.id, selected: m.id === data.model }, modelLabel(m)))),
                        t.button({ type: "button", className: "btn transparent secondary", onclick: () => (data.changingModel = false) }, t.span({ className: "txt" }, "Cancel")),
                        t.button({ className: "btn" }, t.span({ className: "txt" }, "Use this model")))
                    : t.div(null,
                        t.p({ className: "txt-hint m-0" },
                            "Every bucket is embedded with ", t.code(null, st.model), ". ",
                            t.a({ href: "#", onclick: (e) => { e.preventDefault(); openModel(); } }, "Change")),
                        st.embedded < st.chunks ? progress(st) : null));
        }

        return t.div({ className: "page-search-list", onunmount: () => clearTimeout(timer) }, page("Search", [t.div({ className: "breadcrumb-item" }, "Search")], () => {
            if (data.loading) return t.div({ className: "block txt-center" }, t.span({ className: "loader lg" }));
            return t.div(null,
                t.p({ className: "m-t-0" },
                    "Buckets of content your app searches, and adds to its AI prompts with ", t.code(null, "enrich"), ". ",
                    "An app makes the bucket it needs; load it here by dropping files on it, pasting text, or having it follow a collection."),
                modelCard(),
                data.buckets.length
                    ? t.table({ className: "search-table m-b-base" },
                        t.thead(null, t.tr(null, t.th(null, "Bucket"), t.th(null, "Made by"), t.th({ className: "search-num" }, "Documents"),
                            t.th({ className: "search-num" }, "Chunks"), t.th({ className: "search-num" }, "Size"), t.th(null, "Content"), t.th(null, "Status"))),
                        t.tbody(null, ...data.buckets.map((b) => t.tr({ className: "link", onclick: () => (window.location.hash = "#/search/" + encodeURIComponent(b.name)) },
                            t.td(null, t.strong(null, b.title || b.name), b.title ? t.div({ className: "txt-hint txt-sm" }, b.name) : null),
                            t.td(null, b.owner || ""),
                            t.td({ className: "search-num" }, b.documents),
                            t.td({ className: "search-num" }, b.chunks),
                            t.td({ className: "search-num" }, fmtBytes(b.bytes)),
                            t.td(null, b.source ? ["follows ", t.code(null, b.source.collection)] : "files and text"),
                            t.td(null, b.documents === 0 ? t.span({ className: "label warning" }, "empty")
                                : b.errors ? t.span({ className: "label danger" }, b.errors + " failed")
                                    : b.pending ? t.span({ className: "label warning" }, b.pending + " waiting")
                                        : t.span({ className: "label success" }, "ready"))))))
                    : t.div({ className: "block txt-center txt-hint p-base" }, "No buckets yet."),
                () => data.creating
                    ? t.form({ className: "grid m-b-base", onsubmit: create },
                        t.div({ className: "col-lg-4" }, t.div({ className: "field" }, t.label({}, "Name"),
                            t.input({ type: "text", required: true, pattern: "[A-Za-z0-9][A-Za-z0-9_\\-]{0,63}", placeholder: "docs",
                                value: () => data.name, oninput: (e) => (data.name = e.target.value) }))),
                        t.div({ className: "col-lg-8" }, t.div({ className: "field" }, t.label({}, "Title"),
                            t.input({ type: "text", value: () => data.title, oninput: (e) => (data.title = e.target.value) }))),
                        t.div({ className: "col-lg-12" }, t.div({ className: "field" }, t.label({}, "What belongs in it"),
                            t.input({ type: "text", value: () => data.description, oninput: (e) => (data.description = e.target.value) }))),
                        t.div({ className: "col-lg-12 flex" },
                            t.button({ type: "button", className: "btn transparent secondary", onclick: () => (data.creating = false) }, t.span({ className: "txt" }, "Cancel")),
                            t.button({ className: "btn" }, t.span({ className: "txt" }, "Create bucket"))))
                    : t.button({ type: "button", className: "btn secondary sm m-b-base", onclick: () => (data.creating = true) },
                        t.i({ className: "ri-add-line", ariaHidden: true }), t.span({ className: "txt" }, "New bucket")),
                data.buckets.length && data.status?.model ? t.div(null,
                    t.hr(),
                    t.div({ className: "txt-lg m-b-sm" }, "Search every bucket"),
                    searchBox([])) : null,
            );
        }));
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
            loading: true, bucket: null, documents: [], over: false, adding: "",
            pasteOpen: false, pasteName: "", pasteText: "",
            settingsOpen: false, edit: {}, collections: [],
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

        async function openSettings() {
            const b = data.bucket;
            data.edit = {
                title: b.title, description: b.description, maxDocuments: b.maxDocuments, maxMB: Math.round(b.maxBytes / (1 << 20)),
                collection: b.source?.collection || "", fields: (b.source?.fields || []).join(", "), filter: b.source?.filter || "",
            };
            data.settingsOpen = true;
            try {
                data.collections = (await app.pb.collections.getFullList({ requestKey: null })).filter((c) => !c.system && !c.name.startsWith("_"));
            } catch (_) {
                data.collections = [];
            }
        }

        async function saveSettings(e) {
            e.preventDefault();
            const b = data.bucket, x = data.edit;
            const body = { name, title: x.title, description: x.description, maxDocuments: parseInt(x.maxDocuments) || 0, maxBytes: (parseInt(x.maxMB) || 0) * (1 << 20) };
            const fields = x.fields.split(",").map((f) => f.trim()).filter(Boolean);
            const source = x.collection ? { collection: x.collection, fields, filter: x.filter.trim() } : { collection: "" };
            if (JSON.stringify(source) !== JSON.stringify(b.source || { collection: "" })) body.source = source;
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
        const field = (label, input, cls = "col-lg-12") => t.div({ className: cls }, t.div({ className: "field" }, t.label({}, label), input));
        const text = (key, props = {}) => t.input({ type: "text", ...props, value: () => data.edit[key], oninput: (e) => (data.edit[key] = e.target.value) });

        return t.div({ className: "page-search-bucket", onunmount: () => clearTimeout(timer) }, page(name, crumbs, () => {
            if (data.loading) return t.div({ className: "block txt-center" }, t.span({ className: "loader lg" }));
            const b = data.bucket;
            if (!b) return t.div({ className: "block txt-center txt-hint p-base" }, "No bucket named ", t.code(null, name), ".");
            return t.div(null,
                b.description ? t.div({ className: "alert info m-b-sm" }, t.div({ className: "content" },
                    b.owner ? t.strong(null, b.owner + " wants: ") : null, b.description)) : null,
                t.p({ className: "txt-hint m-t-0" },
                    `${b.documents} document${b.documents === 1 ? "" : "s"}, ${b.chunks} chunks, ${fmtBytes(b.bytes)}. `,
                    b.source ? ["Follows ", t.code(null, b.source.collection), ` (${b.source.fields.join(", ")}${b.source.filter ? "; " + b.source.filter : ""}). `] : "",
                    t.a({ href: "#", onclick: (e) => { e.preventDefault(); openSettings(); } }, "Settings")),

                () => data.settingsOpen ? t.form({ className: "grid m-b-base", onsubmit: saveSettings },
                    field("Title", text("title"), "col-lg-6"),
                    field("What belongs in it", text("description"), "col-lg-6"),
                    field("Follow a collection", t.select({ onchange: (e) => (data.edit.collection = e.target.value) },
                        t.option({ value: "", selected: !data.edit.collection }, "None (files and text only)"),
                        ...data.collections.map((c) => t.option({ value: c.name, selected: c.name === data.edit.collection }, c.name))), "col-lg-4"),
                    field("Fields to search (the first is the title)", text("fields", { placeholder: "title, body" }), "col-lg-4"),
                    field("Only records matching", text("filter", { placeholder: "published = true" }), "col-lg-4"),
                    field("Most documents", t.input({ type: "number", min: 0, value: () => data.edit.maxDocuments, oninput: (e) => (data.edit.maxDocuments = e.target.value) }), "col-lg-6"),
                    field("Most text (MB)", t.input({ type: "number", min: 0, value: () => data.edit.maxMB, oninput: (e) => (data.edit.maxMB = e.target.value) }), "col-lg-6"),
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
                            t.td(null, d.record ? t.a({ href: recordHref(d.record) }, d.name) : d.name,
                                d.error ? t.div({ className: "txt-danger txt-sm" }, d.error) : null),
                            t.td(null, d.record ? "record" : d.kind),
                            t.td({ className: "search-num" }, fmtBytes(d.size)),
                            t.td({ className: "search-num" }, d.chunks),
                            t.td({ className: "search-num" }, fmtCost(d.costMicroUSD)),
                            t.td(null, statusLabel(d.status, d.error)),
                            t.td({ className: "txt-right" },
                                d.status === "error" ? t.button({ type: "button", className: "btn xs transparent secondary", onclick: () => reindex([d.name]) }, "Retry") : null,
                                d.record ? null : t.button({ type: "button", className: "btn xs transparent danger", title: "Remove", onclick: () => remove(d) },
                                    t.i({ className: "ri-delete-bin-7-line", ariaHidden: true })))))))
                    : t.div({ className: "block txt-center txt-hint p-base" }, "Nothing in this bucket yet."),

                t.hr(),
                t.div({ className: "txt-lg m-b-sm" }, "Try a search"),
                searchBox([name]),
            );
        }));
    });
})();
