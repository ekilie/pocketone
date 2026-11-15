// Docs pages swap #content and #page-footer without a reload (Datastar; see
// _private/docs-footer.js). This keeps up with each swap: it sets the tab
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

  const load = () => {
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

  let timer
  new MutationObserver(() => {
    clearTimeout(timer)
    timer = setTimeout(load, 50)
  }).observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-giscus-term', 'data-title'] })
  load()
})()
