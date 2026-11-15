// The footer under each docs page: the "Edit this page" link and the giscus comments slot (assets/docs.js loads
// giscus into it, and keeps
// the tab title in step).
//
// Comments: one thread per page in the Q&A category, matched by an exact term. A plugin's page and its guides share
// the plugin's thread. Front matter `discussion: <term>` picks another term; `discussion: false` turns comments off.
// The docs home and the plugin index have none.
//
// Docs links swap only #content, and a Datastar request gets the page without its layouts. So besides
// ctx.docsFooter() for the docs layout, this sends the footer (carrying the page title) along with
// each swap.
module.exports = function () {
  const esc = (s) =>
    String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c])

  const term = (ctx) => {
    const path = ctx.req.url.pathname.replace(/\/+$/, '')
    const d = ctx.meta('discussion')
    if (d === false || d === 'false') return ''
    if (d) return String(d)
    const plugin = path.match(/^\/docs\/plugins\/([^/]+)/)
    if (plugin) return `pocketone.dev/docs/plugins/${plugin[1]}`
    if (path === '/docs' || path === '/docs/plugins') return ''
    return `pocketone.dev${path}`
  }

  const footer = (ctx) => {
    let html = ''
    const source = ctx.meta('source')
    if (source) {
      html += `<p class="edit-page"><a href="https://github.com/pockethost/pocketone/edit/main/${esc(source)}" target="_blank">Edit this page on GitHub</a></p>`
    }
    const t = term(ctx)
    if (t) {
      html += `<section class="comments" data-giscus-term="${esc(t)}"><h2>Questions and comments</h2><p>Comments are a <a href="https://github.com/pockethost/pocketone/discussions/categories/q-a" target="_blank">GitHub Discussions</a> thread; sign in with GitHub to post.</p><div class="giscus"></div></section>`
    }
    return html
  }

  return {
    name: 'docs-footer',
    extendContext(ctx) {
      ctx.docsFooter = () => footer(ctx)
    },
    transform({ html, ctx, file }) {
      if (file !== ctx.route.file || ctx.req.header('Datastar-Request') !== 'true') return html
      if (!ctx.req.url.pathname.startsWith('/docs')) return html
      const title = esc(ctx.meta('title') || 'PocketOne')
      ctx.datastar.patchElements(`<div id="page-footer" data-title="${title}">${footer(ctx)}</div>`)
      return html
    },
  }
}
