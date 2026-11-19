// PocketOne branding for the superuser dashboard: logo, favicon and the version link in the footer.
const logo = app.pb.buildURL("/_/extensions/pocketone/logo.svg");

app.store.mainLogo = logo;
app.store.headerLogo = logo;
app.store.favicon = logo;

// The version comes from the server's X-Powered-By header ("PocketOne 0.0.2") on each page load, because the
// dashboard caches this file for days and a version baked into it would go stale after an upgrade.
let version = "";
try {
    const res = await fetch(app.pb.buildURL("/api/health"), { cache: "no-store" });
    const m = /^PocketOne\s+(\S+)/.exec(res.headers.get("X-Powered-By") || "");
    if (m) {
        version = /^\d/.test(m[1]) ? "v" + m[1] : m[1];
    }
} catch (err) {
    console.warn("PocketOne version lookup failed:", err);
}

app.store.creditLinks = app.store.creditLinks.map((link) => {
    if (!/^PocketBase /.test(link.label)) {
        return link;
    }
    return {
        ...link,
        href: "https://github.com/pocketonehq/pocketone/releases",
        label: version ? `PocketOne ${version}` : "PocketOne",
    };
});
