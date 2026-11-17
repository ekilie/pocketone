// Docs pages swap #content and #page-footer without a reload (Datastar; see
// _private/docs-footer.js). This keeps up with each swap: it highlights the
// new page's code blocks (Prism only runs once, on page load), sets the tab
// title from #page-footer's data-title, and loads giscus into the footer's
// <section data-giscus-term> slot whenever it comes back empty or its term
// changes.
;(() => {
  const config = {
    repo: 'pockethost/pocketone',
    'repo-id': 'R_kgDONaSBnA',
    category: 'Q&A',
    'category-id': 'DIC_kwDONaSBnM4DK7Ec',
    mapping: 'specific',
    strict: '1',
    'reactions-enabled': '1',
    'emit-metadata': '0',
    'input-position': 'top',
    theme: 'light',
    lang: 'en',
    loading: 'lazy',
  }

  const highlight = () => {
    if (!window.Prism) return
    document.querySelectorAll('#content pre > code[class*="language-"]:not([data-highlighted])').forEach((code) => {
      code.dataset.highlighted = ''
      // Prism's own load-time pass did this one (it wraps the block in its copy toolbar)
      if (code.parentElement.parentElement?.classList.contains('code-toolbar')) return
      window.Prism.highlightElement(code)
    })
  }

  const load = () => {
    highlight()
    const title = document.getElementById('page-footer')?.dataset.title
    if (title && document.title !== title) document.title = title
    const slot = document.querySelector('[data-giscus-term]')
    const box = slot?.querySelector('.giscus')
    if (!box) return
    const term = slot.dataset.giscusTerm
    // A swap drops this attribute (and the iframe), so it marks "already loaded for this term".
    if (box.dataset.loaded === term) return
    box.dataset.loaded = term
    box.replaceChildren()
    const s = document.createElement('script')
    s.src = 'https://giscus.app/client.js'
    s.async = true
    s.crossOrigin = 'anonymous'
    for (const [k, v] of Object.entries({ ...config, term })) s.setAttribute(`data-${k}`, v)
    box.append(s)
  }

  // A nav link to a page with another nav (into an extension site such as Pages, or back out) loads the whole page,
  // so the sidebar changes too: a swap replaces only #content. The swap script skips links with a target.
  const toc = document.getElementById('toc')
  const navs = new Set((toc?.dataset.navs || '').split(',').filter(Boolean))
  const navOf = (href) => {
    const m = href.match(/^\/docs\/extensions\/([^/?#]+)/)
    return m && navs.has(m[1]) ? m[1] : ''
  }
  document.querySelectorAll('.docs-toc-navigation a[href^="/"]').forEach((a) => {
    if (navOf(a.getAttribute('href')) !== (toc?.dataset.nav || '')) a.target = '_self'
  })

  let timer
  new MutationObserver(() => {
    clearTimeout(timer)
    timer = setTimeout(load, 50)
  }).observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-giscus-term', 'data-title'] })
  load()
})()
