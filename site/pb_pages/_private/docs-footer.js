// The footer under each docs page: the "Edit this page" link and the giscus comments slot (assets/docs.js loads
// giscus into it, and keeps
// the tab title in step).
//
// Comments: one thread per page in the Q&A category, matched by an exact term. An extension's page and its guides (the
// extra .md files beside its README; site/sync gives them `discussion:`) share the extension's thread; each page of an
// extension's docs (ext/<name>/docs) have their own. Pages that moved from /docs/plugins/ and /docs/writing-a-plugin keep
// their old terms, so their existing threads follow them. Front matter `discussion: <term>` picks another term; `discussion: false` turns comments off.
// The docs home and the extension index have none.
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
    const ext = path.match(/^\/docs\/extensions\/([^/]+)$/)
    if (ext) return `pocketone.dev/docs/plugins/${ext[1]}` // old term: keeps threads from before the rename
    if (path === '/docs/writing-an-extension') return 'pocketone.dev/docs/writing-a-plugin'
    if (path === '/docs' || path === '/docs/extensions') return ''
    return `pocketone.dev${path}`
  }

  const footer = (ctx) => {
    let html = ''
    const source = ctx.meta('source')
    if (source) {
      html += `<p class="edit-page"><a href="https://github.com/pocketonehq/pocketone/edit/main/${esc(source)}" target="_blank">Edit this page on GitHub</a></p>`
    }
    const t = term(ctx)
    if (t) {
      html += `<section class="comments" data-giscus-term="${esc(t)}"><h2>Questions and comments</h2><p>Comments are a <a href="https://github.com/pocketonehq/pocketone/discussions/categories/q-a" target="_blank">GitHub Discussions</a> thread; sign in with GitHub to post.</p><div class="giscus"></div></section>`
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
