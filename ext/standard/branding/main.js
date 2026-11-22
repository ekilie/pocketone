// PocketOne branding for the superuser dashboard: logo, favicon, the docs link and the version in the footer.
// ext/standard serves this file with the running PocketOne version written in below.
const version = "__POCKETONE_VERSION__";
const logo = app.pb.buildURL("/_/extensions/pocketone/logo.svg");

app.store.mainLogo = logo;
app.store.headerLogo = logo;
app.store.favicon = logo;

app.store.creditLinks = app.store.creditLinks.map((link) => {
    if (link.label === "Docs") {
        return { ...link, href: "https://pocketone.dev/docs" };
    }
    if (/^PocketBase\b/.test(link.label)) {
        return {
            ...link,
            href: "https://github.com/pocketonehq/pocketone/releases",
            label: version ? `PocketOne ${version}` : "PocketOne",
        };
    }
    return link;
});
