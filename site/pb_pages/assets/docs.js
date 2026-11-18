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

  // A nav link to a page with another nav (into an extension's docs such as Pages, or back out) loads the whole page,
  // so the sidebar changes too: a swap replaces only #content. The swap script skips links with a target.
  const toc = document.getElementById('toc')
  const navs = new Set((toc?.dataset.navs || '').split(',').filter(Boolean))
  // the same choice as _private/docs-nav.js
  const navOf = (href) => {
    const path = href.split(/[?#]/)[0]
    const m = path.match(/^\/docs\/extensions\/([^/]+)/)
    if (m && navs.has(m[1]) && !new RegExp(`^/docs/extensions/${m[1]}/extension(/|$)`).test(path)) return m[1]
    if (/^\/docs\/extensions(\/|$)/.test(path)) return 'extensions'
    if (/^\/docs\/contracts(\/|$)/.test(path)) return 'contracts'
    return ''
  }
  // how deep a nav is: the main nav, a section (Contracts, Extensions), an extension's own docs
  const depth = (nav) => (nav === '' ? 0 : nav === 'contracts' || nav === 'extensions' ? 1 : 2)
  const current = toc?.dataset.nav || ''
  document.querySelectorAll('.docs-toc-navigation a[href^="/"]').forEach((a) => {
    const to = navOf(a.getAttribute('href'))
    if (to === current) return
    a.target = '_self'
    if (depth(to) > depth(current)) a.classList.add('nav-in') // a link that slides into a deeper nav
  })
  // Coming from another nav, slide this one in: from the right going deeper, from the left coming back.
  try {
    const prev = sessionStorage.getItem('docs-nav')
    if (toc && prev !== null && prev !== current) toc.classList.add(depth(current) > depth(prev) ? 'nav-slide-in' : 'nav-slide-back')
    sessionStorage.setItem('docs-nav', current)
  } catch (e) {}

  let timer
  new MutationObserver(() => {
    clearTimeout(timer)
    timer = setTimeout(load, 50)
  }).observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-giscus-term', 'data-title'] })
  load()
})()
