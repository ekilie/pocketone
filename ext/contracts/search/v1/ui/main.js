// The Search screen for the PocketBase dashboard, for the search/v1 contract (PocketOne ext/contracts/search/v1): every
// search bucket of the app, its sources (files, pasted text, folders, zips, web pages, Git repositories, collections)
// and their documents, a drop zone and forms to add them, a search to try, and the app's embedding model. It calls only search/v1 (and llm.embed/v1's model list for
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
        .search-num { text-align: right !important; font-variant-numeric: tabular-nums; white-space: nowrap; }`;
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
                            t.td(null, `${b.sources} source${b.sources === 1 ? "" : "s"}`),
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

    // folderFiles is a dropped folder's files, by their path inside it
    async function folderFiles(entry) {
        const out = [];
        const go = async (dir, prefix) => {
            const reader = dir.createReader();
            let batch;
            do {
                batch = await new Promise((res) => reader.readEntries(res, () => res([])));
                for (const e of batch) {
                    if (e.isDirectory) await go(e, prefix + e.name + "/");
                    else if (e.isFile) out.push({ file: await new Promise((res) => e.file(res, () => res(null))), path: prefix + e.name });
                }
            } while (batch.length);
        };
        await go(entry, "");
        return out.filter((f) => f.file);
    }

    // skipped mirrors what the server leaves out of folders, zips and repositories
    const skipDirs = new Set(["node_modules", "vendor", "__pycache__"]);
    const lockfiles = new Set(["package-lock.json", "yarn.lock", "pnpm-lock.yaml", "bun.lockb", "go.sum", "Cargo.lock", "composer.lock", "Gemfile.lock", "poetry.lock", "uv.lock"]);
    const skipped = (p) => {
        const parts = p.split("/");
        return parts.some((x, i) => x.startsWith(".") || (i < parts.length - 1 && skipDirs.has(x))) || lockfiles.has(parts[parts.length - 1]);
    };

    async function sha256(blob) {
        const buf = await crypto.subtle.digest("SHA-256", await blob.arrayBuffer());
        return [...new Uint8Array(buf)].map((b) => b.toString(16).padStart(2, "0")).join("");
    }

    // uploadRaw sends a file as the request body (a zip, or a file the browser can't read as text)
    async function uploadRaw(bucket, name, file, replace) {
        const q = new URLSearchParams({ bucket, name });
        if (replace) q.set("replace", replace);
        const res = await fetch(app.pb.buildURL(api + "/upload?" + q), {
            method: "POST", body: file, headers: { Authorization: app.pb.authStore.token, "Content-Type": "application/octet-stream" },
        });
        const body = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error(body.message || res.statusText);
        return body;
    }

    const kindIcon = { file: "ri-file-text-line", text: "ri-text", folder: "ri-folder-line", zip: "ri-file-zip-line", page: "ri-global-line", git: "ri-git-branch-line", collection: "ri-database-2-line" };
    const fmtWhen = (iso) => (iso ? new Date(iso).toLocaleString() : "");

    app.routes.superuserOnly("#/search/{name}", (route) => {
        const name = decodeURIComponent(route.params.name);
        const data = store({
            loading: true, bucket: null, sources: [], over: false, adding: "",
            form: "", // "paste" | "url" | "follow" | "settings"
            pasteName: "", pasteText: "", url: "", every: "day", follow: { collection: "", fields: "", filter: "" },
            edit: {}, collections: [], open: {}, docs: {},
        });
        let timer = null;

        async function load() {
            try {
                const [b, l] = await Promise.all([send("/bucket?name=" + encodeURIComponent(name)), send("/sources?bucket=" + encodeURIComponent(name))]);
                data.bucket = b;
                data.sources = l.sources || [];
                for (const src of data.sources) if (data.open[src.name]) loadDocs(src.name);
            } catch (err) {
                app.checkApiError(err);
            }
            data.loading = false;
            clearTimeout(timer);
            const busy = data.bucket?.pending || data.sources.some((x) => x.status === "reading");
            if (busy && document.querySelector(".page-search-bucket")) timer = setTimeout(load, 2000);
        }

        async function loadDocs(source) {
            try {
                const d = await send("/documents?bucket=" + encodeURIComponent(name) + "&source=" + encodeURIComponent(source));
                data.docs = { ...data.docs, [source]: d.documents || [] };
            } catch (_) {}
        }

        function toggle(source) {
            data.open = { ...data.open, [source]: !data.open[source] };
            if (data.open[source]) loadDocs(source);
        }

        const existing = (n) => data.sources.find((x) => x.name === n);

        // confirmAdd asks before replacing a source of the same name, and before adding content that's already here
        // under another name; it returns false to skip
        async function confirmAdd(n, file) {
            const same = existing(n);
            if (same) return confirm(`Replace ${n} (${same.documents} document${same.documents === 1 ? "" : "s"})? Unchanged files cost nothing; files it no longer has are removed.`);
            if (file) {
                const h = await sha256(file);
                const dup = data.sources.find((x) => x.hash === h);
                if (dup) return confirm(`${n} is the same as ${dup.name}, already in this bucket. Add it again anyway?`);
            }
            return true;
        }

        // addFile adds a loose file or a zip as its own source (replacing the one named replace, when given)
        async function addFile(file, replace) {
            if (!replace && !(await confirmAdd(file.name, file))) return;
            if (/\.zip$/i.test(file.name)) {
                await uploadRaw(name, file.name, file, replace);
            } else {
                if (file.size > MAX_FILE) throw new Error("over 10 MB");
                await send("/add", { bucket: name, name: file.name, text: await file.text(), kind: "file", replace });
            }
        }

        // addFolder sends a dropped folder file by file, then prunes what it no longer has
        async function addFolder(entry, replace) {
            const folder = entry.name;
            if (!replace && existing(folder) && !(await confirmAdd(folder))) return;
            const files = (await folderFiles(entry)).filter((f) => !skipped(f.path) && f.file.size <= 1 << 20);
            const keep = [];
            let failed = 0;
            for (const [i, f] of files.entries()) {
                data.adding = `${folder}: ${i + 1} of ${files.length}, ${f.path}`;
                try {
                    const text = await f.file.text();
                    if (text.includes("\u0000")) continue; // binary
                    await send("/put", { bucket: name, source: folder, path: f.path, text, replace: i === 0 ? replace : undefined });
                    keep.push(f.path);
                } catch (_) {
                    failed++;
                }
            }
            await send("/prune", { bucket: name, source: folder, keep });
            if (failed) app.toasts.error(`${folder}: ${failed} file${failed === 1 ? "" : "s"} not added.`);
        }

        async function onDrop(e) {
            e.preventDefault();
            data.over = false;
            const entries = [...(e.dataTransfer.items || [])].map((i) => i.webkitGetAsEntry?.()).filter(Boolean);
            const files = entries.length ? [] : [...(e.dataTransfer.files || [])];
            try {
                for (const en of entries) {
                    data.adding = "Adding " + en.name;
                    if (en.isDirectory) await addFolder(en);
                    else await addFile(await new Promise((res, rej) => en.file(res, rej)));
                }
                for (const f of files) {
                    data.adding = "Adding " + f.name;
                    await addFile(f);
                }
            } catch (err) {
                app.toasts.error(errText(err));
            }
            data.adding = "";
            load();
        }

        // newVersion picks a file (or zip) that replaces a source, whatever it's called now
        function newVersion(src) {
            const input = document.createElement("input");
            input.type = "file";
            input.onchange = async () => {
                const f = input.files[0];
                if (!f) return;
                data.adding = `Replacing ${src.name} with ${f.name}`;
                try {
                    await addFile(f, src.name);
                } catch (err) {
                    app.toasts.error(errText(err));
                }
                data.adding = "";
                load();
            };
            input.click();
        }

        async function submit(path, body, after) {
            try {
                await send(path, body);
                data.form = "";
                after?.();
                load();
            } catch (err) {
                app.toasts.error(errText(err));
            }
        }

        function remove(src) {
            app.modals.confirm(`Take ${src.name} out of the bucket, with its ${src.documents} document${src.documents === 1 ? "" : "s"}?`, () =>
                submit("/remove", { bucket: name, sources: [src.name] }));
        }

        async function openForm(form) {
            data.form = data.form === form ? "" : form;
            if (form === "settings") {
                const b = data.bucket;
                data.edit = { title: b.title, description: b.description, maxDocuments: b.maxDocuments, maxMB: Math.round(b.maxBytes / (1 << 20)) };
            }
            if (form === "follow") {
                try {
                    data.collections = (await app.pb.collections.getFullList({ requestKey: null })).filter((c) => !c.system && !c.name.startsWith("_"));
                } catch (_) {
                    data.collections = [];
                }
            }
        }

        function deleteBucket() {
            app.modals.confirm(`Delete the bucket ${name} and everything in it? Apps that search it will find nothing until it's filled again.`, async () => {
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
        const bound = (obj, key, props = {}) => t.input({ type: "text", ...props, value: () => obj[key], oninput: (e) => (obj[key] = e.target.value) });
        const formButtons = (label) => t.div({ className: "col-lg-12 flex" }, t.div({ className: "m-r-auto" }),
            t.button({ type: "button", className: "btn transparent secondary", onclick: () => (data.form = "") }, t.span({ className: "txt" }, "Cancel")),
            t.button({ className: "btn" }, t.span({ className: "txt" }, label)));

        function sourceStatus(src) {
            if (src.status === "error") return t.span({ className: "label danger", title: src.error }, "error");
            if (src.status === "reading") return t.span({ className: "label warning" }, t.span({ className: "loader sm m-r-5" }), "reading");
            if (src.pending) return t.span({ className: "label warning" }, `${src.pending} waiting`);
            return t.span({ className: "label success" }, "ready");
        }

        function sourceRows(src) {
            const refreshable = ["page", "git", "collection", "zip"].includes(src.kind);
            const rows = [t.tr(null,
                t.td(null,
                    t.a({ href: "#", onclick: (e) => { e.preventDefault(); toggle(src.name); } },
                        t.i({ className: (data.open[src.name] ? "ri-arrow-down-s-line" : "ri-arrow-right-s-line") + " m-r-5", ariaHidden: true }),
                        t.i({ className: (kindIcon[src.kind] || "ri-file-line") + " m-r-5", ariaHidden: true }), src.name),
                    src.kind === "collection" ? t.div({ className: "txt-hint txt-sm" }, `${src.fields.join(", ")}${src.filter ? "; " + src.filter : ""}`) : null,
                    src.error ? t.div({ className: "txt-danger txt-sm" }, src.error) : null),
                t.td(null, src.kind),
                t.td({ className: "search-num" }, src.documents),
                t.td({ className: "search-num" }, fmtBytes(src.bytes)),
                t.td({ className: "search-num" }, fmtCost(src.costMicroUSD)),
                t.td(null, sourceStatus(src),
                    src.refreshed ? t.div({ className: "txt-hint txt-sm" }, "read " + fmtWhen(src.refreshed)) : null,
                    src.next ? t.div({ className: "txt-hint txt-sm" }, "next " + fmtWhen(src.next)) : null),
                t.td({ className: "txt-right", style: "white-space: nowrap" },
                    refreshable ? t.button({ type: "button", className: "btn xs transparent secondary", title: "Read it again now",
                        onclick: () => submit("/refresh", { bucket: name, source: src.name }) }, t.i({ className: "ri-refresh-line", ariaHidden: true })) : null,
                    src.kind === "file" || src.kind === "zip" ? t.button({ type: "button", className: "btn xs transparent secondary", title: "Upload a new version",
                        onclick: () => newVersion(src) }, t.i({ className: "ri-upload-2-line", ariaHidden: true })) : null,
                    t.button({ type: "button", className: "btn xs transparent danger", title: "Remove", onclick: () => remove(src) },
                        t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }))))];
            if (data.open[src.name]) {
                const docs = data.docs[src.name];
                rows.push(t.tr(null, t.td({ colSpan: 7, style: "padding-left: 40px" },
                    !docs ? t.span({ className: "loader sm" }) : !docs.length ? t.span({ className: "txt-hint" }, "No documents.")
                        : t.table({ className: "search-table" }, t.tbody(null, ...docs.slice(0, 200).map((d) => t.tr(null,
                            t.td(null, d.record ? t.a({ href: recordHref(d.record) }, d.name) : d.name, d.error ? t.div({ className: "txt-danger txt-sm" }, d.error) : null),
                            t.td({ className: "search-num" }, d.chunks + " chunks"),
                            t.td({ className: "search-num" }, fmtBytes(d.size)),
                            t.td(null, statusLabel(d.status, d.error))))),
                        docs.length > 200 ? t.tr(null, t.td({ className: "txt-hint" }, `and ${docs.length - 200} more`)) : null))));
            }
            return rows;
        }

        return t.div({ className: "page-search-bucket", onunmount: () => clearTimeout(timer) }, page(name, crumbs, () => {
            if (data.loading) return t.div({ className: "block txt-center" }, t.span({ className: "loader lg" }));
            const b = data.bucket;
            if (!b) return t.div({ className: "block txt-center txt-hint p-base" }, "No bucket named ", t.code(null, name), ".");
            return t.div(null,
                b.description ? t.div({ className: "alert info m-b-sm" }, t.div({ className: "content" },
                    b.owner ? t.strong(null, b.owner + " wants: ") : null, b.description)) : null,
                t.p({ className: "txt-hint m-t-0" },
                    `${b.sources} source${b.sources === 1 ? "" : "s"}, ${b.documents} documents, ${b.chunks} chunks, ${fmtBytes(b.bytes)}. `,
                    t.a({ href: "#", onclick: (e) => { e.preventDefault(); openForm("settings"); } }, "Settings")),

                () => data.form === "settings" ? t.form({ className: "grid m-b-base", onsubmit: (e) => {
                    e.preventDefault();
                    const x = data.edit;
                    submit("/update", { name, title: x.title, description: x.description, maxDocuments: parseInt(x.maxDocuments) || 0, maxBytes: (parseInt(x.maxMB) || 0) * (1 << 20) });
                } },
                    field("Title", bound(data.edit, "title"), "col-lg-6"),
                    field("What belongs in it", bound(data.edit, "description"), "col-lg-6"),
                    field("Most documents", t.input({ type: "number", min: 0, value: () => data.edit.maxDocuments, oninput: (e) => (data.edit.maxDocuments = e.target.value) }), "col-lg-6"),
                    field("Most text (MB)", t.input({ type: "number", min: 0, value: () => data.edit.maxMB, oninput: (e) => (data.edit.maxMB = e.target.value) }), "col-lg-6"),
                    t.div({ className: "col-lg-12 flex" },
                        t.button({ type: "button", className: "btn sm transparent danger", onclick: deleteBucket },
                            t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }), t.span({ className: "txt" }, "Delete bucket")),
                        t.div({ className: "m-r-auto" }),
                        t.button({ type: "button", className: "btn transparent secondary", onclick: () => (data.form = "") }, t.span({ className: "txt" }, "Cancel")),
                        t.button({ className: "btn" }, t.span({ className: "txt" }, "Save")))) : null,

                t.div({
                    className: () => `search-dropzone m-b-sm ${data.over ? "over" : ""}`,
                    ondragover: (e) => { e.preventDefault(); data.over = true; },
                    ondragleave: () => (data.over = false),
                    ondrop: onDrop,
                },
                    () => data.adding ? t.span(null, t.span({ className: "loader sm m-r-5" }), data.adding) : t.span(null,
                        t.i({ className: "ri-upload-cloud-line", ariaHidden: true }),
                        " Drop files, folders or zips here, ",
                        t.label({ className: "link-primary", style: "cursor: pointer" }, "choose files",
                            t.input({ type: "file", multiple: true, hidden: true, onchange: async (e) => {
                                for (const f of [...e.target.files]) {
                                    data.adding = "Adding " + f.name;
                                    try { await addFile(f); } catch (err) { app.toasts.error(`${f.name}: ${errText(err)}`); }
                                }
                                data.adding = "";
                                load();
                            } })),
                        ", ",
                        t.a({ href: "#", className: "link-primary", onclick: (e) => { e.preventDefault(); openForm("paste"); } }, "paste text"),
                        ", ",
                        t.a({ href: "#", className: "link-primary", onclick: (e) => { e.preventDefault(); openForm("url"); } }, "add a URL"),
                        " or ",
                        t.a({ href: "#", className: "link-primary", onclick: (e) => { e.preventDefault(); openForm("follow"); } }, "follow a collection"),
                        ".")),

                () => data.form === "paste" ? t.form({ className: "grid m-b-base", onsubmit: async (e) => {
                    e.preventDefault();
                    const n = data.pasteName.trim();
                    if (!(await confirmAdd(n))) return;
                    submit("/add", { bucket: name, name: n, text: data.pasteText, kind: "text" }, () => (data.pasteName = data.pasteText = ""));
                } },
                    field("Name", bound(data, "pasteName", { required: true, placeholder: "characters.md" })),
                    field("Text", t.textarea({ rows: 8, required: true, value: () => data.pasteText, oninput: (e) => (data.pasteText = e.target.value) })),
                    formButtons("Add")) : null,

                () => data.form === "url" ? t.form({ className: "grid m-b-base", onsubmit: async (e) => {
                    e.preventDefault();
                    const u = data.url.trim();
                    if (!(await confirmAdd(u))) return;
                    submit("/add-url", { bucket: name, url: u, every: data.every }, () => (data.url = ""));
                } },
                    field("A web page or a Git repository", bound(data, "url", { type: "url", required: true, placeholder: "https://pocketone.dev/llms.txt or https://github.com/owner/repo" }), "col-lg-8"),
                    field("Read it again", t.select({ onchange: (e) => (data.every = e.target.value) },
                        ...[["", "Only when I refresh it"], ["hour", "Every hour"], ["day", "Every day"], ["week", "Every week"]].map(([v, l]) =>
                            t.option({ value: v, selected: v === data.every }, l))), "col-lg-4"),
                    t.div({ className: "col-lg-12 txt-hint txt-sm" }, "A repository brings its default branch's text files and its recent commits (no history beyond that, no binaries, lockfiles, node_modules or vendor)."),
                    formButtons("Add URL")) : null,

                () => data.form === "follow" ? t.form({ className: "grid m-b-base", onsubmit: (e) => {
                    e.preventDefault();
                    const f = data.follow;
                    submit("/follow", { bucket: name, collection: f.collection, fields: f.fields.split(",").map((x) => x.trim()).filter(Boolean), filter: f.filter.trim() });
                } },
                    field("Collection", t.select({ required: true, onchange: (e) => (data.follow.collection = e.target.value) },
                        t.option({ value: "" }, "Pick one"),
                        ...data.collections.map((c) => t.option({ value: c.name, selected: c.name === data.follow.collection }, c.name))), "col-lg-4"),
                    field("Fields to search (the first is the title)", bound(data.follow, "fields", { required: true, placeholder: "title, body" }), "col-lg-4"),
                    field("Only records matching", bound(data.follow, "filter", { placeholder: "published = true" }), "col-lg-4"),
                    formButtons("Follow")) : null,

                data.sources.length
                    ? t.div({ style: "overflow-x: auto" }, t.table({ className: "search-table m-b-base" },
                        t.thead(null, t.tr(null, t.th(null, "Source"), t.th(null, "Kind"), t.th({ className: "search-num" }, "Documents"),
                            t.th({ className: "search-num" }, "Size"), t.th({ className: "search-num" }, "Cost"), t.th(null, "Status"), t.th(null, ""))),
                        t.tbody(null, ...data.sources.flatMap(sourceRows))))
                    : t.div({ className: "block txt-center txt-hint p-base" }, "Nothing in this bucket yet."),

                t.hr(),
                t.div({ className: "txt-lg m-b-sm" }, "Try a search"),
                searchBox([name]),
            );
        }));
    });
})();
