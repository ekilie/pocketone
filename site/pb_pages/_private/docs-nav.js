// Which docs nav a page shows:
//   - the pages of an extension with a site of its own (ext/<name>/site) show its nav, (main)/_private/toc-<name>.md,
//     except its extension docs (/docs/extensions/<name>/extension);
//   - every other extension page shows the Extensions nav (toc-extensions), and contract pages the Contracts nav
//     (toc-contracts), which the main nav's Reference section slides into;
//   - everything else shows the main nav (toc).
// site/sync writes those navs and lists the extensions with a site in site-navs.js. The docs layouts load this with
// ctx.resolve, not as a +config.js plugin: a host loads plugins once, at start, so a deploy that changed one wouldn't
// take effect until a restart. assets/docs.js decides the same way which links change the nav.
//
//   const docsNav = ctx.resolve('docs-nav')(ctx)
//   docsNav.name  // '' for the main nav, 'contracts', 'extensions', or an extension site's name
//   docsNav.toc   // the partial to include: 'toc' or 'toc-<name>'
//   docsNav.names // every extension with a site nav of its own
module.exports = (ctx) => {
  let names = []
  try {
    const v = ctx.resolve('site-navs')
    if (Array.isArray(v)) names = v
  } catch (e) {}
  const path = ctx.req.url.pathname
  const m = path.match(/^\/docs\/extensions\/([^/]+)/)
  let name = ''
  // a site's own pages show its nav; its extension docs (/docs/extensions/<name>/extension, site/sync's readmeSlug)
  // are an extension page like any other
  if (m && names.includes(m[1]) && !new RegExp(`^/docs/extensions/${m[1]}/extension(/|$)`).test(path)) name = m[1]
  else if (/^\/docs\/extensions(\/|$)/.test(path)) name = 'extensions'
  else if (/^\/docs\/contracts(\/|$)/.test(path)) name = 'contracts'
  return { name, names, toc: name ? `toc-${name}` : 'toc' }
}
