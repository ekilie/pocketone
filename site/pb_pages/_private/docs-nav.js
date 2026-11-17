// Which docs nav a page shows. The pages of an extension with a site of its own (ext/<name>/site) show its nav,
// (main)/_private/toc-<name>.md, in place of the main one (toc). site/sync writes both and lists those extensions in
// site-navs.js. The docs layouts load this with ctx.resolve, not as a +config.js plugin: a host loads plugins once, at
// start, so a deploy that changed one wouldn't take effect until a restart.
//
//   const docsNav = ctx.resolve('docs-nav')(ctx)
//   docsNav.name  // '' for the main nav, else the extension's name
//   docsNav.toc   // the partial to include: 'toc' or 'toc-<name>'
//   docsNav.names // every extension with a nav of its own
module.exports = (ctx) => {
  let names = []
  try {
    const v = ctx.resolve('site-navs')
    if (Array.isArray(v)) names = v
  } catch (e) {}
  const m = ctx.req.url.pathname.match(/^\/docs\/extensions\/([^/]+)/)
  const name = m && names.includes(m[1]) ? m[1] : ''
  return { name, names, toc: name ? `toc-${name}` : 'toc' }
}
