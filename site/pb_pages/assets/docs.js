// Docs pages swap #content and #page-footer without a reload (Datastar; see
// _private/docs-footer.js). This keeps up with each swap: it turns runs of
// TS, Go and ETS code blocks into tabs, highlights the new page's code blocks
// (Prism only runs once, on page load), sets the tab
// title from #page-footer's data-title, and loads giscus into the footer's
// <section data-giscus-term> slot whenever it comes back empty or its term
// changes.
;(() => {
  const config = {
    repo: 'pocketonehq/pocketone',
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

  // Mark the nav link for the page being viewed (aria-current, styled in site.css), in the sidebar and the mobile
  // drawer. A page the nav doesn't list marks the link it falls under: the longest listed path it starts with.
  const trim = (p) => p.replace(/\/+$/, '') || '/'
  let marked
  const markCurrent = () => {
    const here = trim(location.pathname)
    if (here === marked) return
    marked = here
    const links = [...document.querySelectorAll('.docs-toc-navigation a[href^="/"]')]
    let best = ''
    for (const a of links) {
      const p = trim(a.getAttribute('href').split(/[?#]/)[0])
      if (p !== '/docs' && (here === p || here.startsWith(p + '/')) && p.length > best.length) best = p
    }
    if (here === '/docs') best = '/docs'
    for (const a of links) {
      if (best && trim(a.getAttribute('href').split(/[?#]/)[0]) === best) a.setAttribute('aria-current', 'page')
      else a.removeAttribute('aria-current')
    }
    // keep the marked link in view in the sidebar's own scroll box
    const box = document.querySelector('.docs-nav-scroll')
    const cur = box?.querySelector('a[aria-current="page"]')
    if (box && cur) {
      const b = box.getBoundingClientRect()
      const c = cur.getBoundingClientRect()
      if (c.top < b.top || c.bottom > b.bottom) box.scrollTop += c.top - b.top - b.height / 2 + c.height / 2
    }
  }

  // Code tabs: a TS, Go or ETS code block directly followed by another of those is one example written several ways
  // (ETS when the example needs a page template). Each run becomes a tabbed block. The reader's choice is one of two:
  // TypeScript (the TS or ETS tab: the same preference, so a group has one or the other, never both) or Go. Picking a
  // tab picks that side in every group on the page, and it's remembered. The Markdown needs nothing special: on
  // GitHub and in llms.txt the blocks just follow each other.
  const tabLangs = { ts: 'TS', typescript: 'TS', go: 'Go', ets: 'ETS' }
  const side = (lang) => (lang === 'Go' ? 'go' : 'ts') // TS and ETS are one preference
  const langOf = (el) => {
    const pre = el?.matches?.('pre') ? el : el?.matches?.('div.code-toolbar') ? el.querySelector('pre') : null
    const m = pre?.querySelector('code')?.className.match(/language-(\w+)/)
    return m && tabLangs[m[1]] ? tabLangs[m[1]] : null
  }
  let preferred = 'ts' // 'ts' or 'go'
  try {
    if (localStorage.getItem('code-lang') === 'go') preferred = 'go'
  } catch (e) {}
  const pick = (group, pref) => {
    const tabs = [...group.querySelectorAll(':scope > .code-tabs-bar > button')]
    const on = (tabs.find((b) => side(b.dataset.lang) === pref) || tabs[0]).dataset.lang
    for (const b of tabs) b.setAttribute('aria-selected', String(b.dataset.lang === on))
    for (const p of group.querySelectorAll(':scope > .code-tabs-panel')) p.hidden = p.dataset.lang !== on
  }
  const groupCode = () => {
    const blocks = [...document.querySelectorAll('#content pre, #content div.code-toolbar')].filter(
      (el) => !el.closest('.code-tabs') && !(el.matches('pre') && el.parentElement.matches('div.code-toolbar')),
    )
    const seen = new Set()
    for (const first of blocks) {
      if (seen.has(first) || !langOf(first)) continue
      const run = [first]
      for (let next = first.nextElementSibling; next && langOf(next); next = next.nextElementSibling) {
        // the same side twice (TS twice, or TS and ETS): separate examples
        if (run.some((el) => side(langOf(el)) === side(langOf(next)))) break
        run.push(next)
      }
      run.forEach((el) => seen.add(el))
      if (run.length < 2) continue
      const group = document.createElement('div')
      group.className = 'code-tabs'
      const bar = document.createElement('div')
      bar.className = 'code-tabs-bar'
      bar.setAttribute('role', 'tablist')
      first.before(group)
      group.append(bar)
      for (const el of run) {
        const lang = langOf(el)
        const b = document.createElement('button')
        b.type = 'button'
        b.setAttribute('role', 'tab')
        b.dataset.lang = lang
        b.textContent = lang
        bar.append(b)
        const panel = document.createElement('div')
        panel.className = 'code-tabs-panel'
        panel.setAttribute('role', 'tabpanel')
        panel.dataset.lang = lang
        panel.append(el)
        group.append(panel)
      }
      pick(group, preferred)
    }
  }
  document.addEventListener('click', (e) => {
    const b = e.target.closest?.('.code-tabs-bar > button')
    if (!b) return
    preferred = side(b.dataset.lang)
    try {
      localStorage.setItem('code-lang', preferred)
    } catch (err) {}
    const before = b.getBoundingClientRect().top // keep the clicked tab still while other groups change height
    document.querySelectorAll('.code-tabs').forEach((g) => pick(g, preferred))
    window.scrollBy(0, b.getBoundingClientRect().top - before)
  })

  // On this page: an outline of the page's H2s (and the H3s under the one being read), built from #content after each
  // swap. A sticky rail beside the article on wide screens, a box you open above it on narrow ones. Pages with more
  // than 30 headings (the API references) get a filter that searches every H2 and H3. Shown when a page has 3+ H2s.
  const wide = window.matchMedia('(width >= 80rem)')
  let spyItems = []
  const spy = () => {
    if (!spyItems.length) return
    let at = spyItems[0]
    for (const it of spyItems) {
      if (it.h.getBoundingClientRect().top > 96) break
      at = it
    }
    for (const it of spyItems) {
      it.a.classList.toggle('active', it === at)
      if (it.sub) it.li.classList.toggle('open', it === at || it === at.parent)
    }
  }
  const outline = () => {
    const main = document.querySelector('.docs-main')
    const content = document.getElementById('content')
    if (!main || !content) return
    let box = document.getElementById('page-outline')
    const hs = [...content.querySelectorAll('h2[id], h3[id]')].filter((h) => !h.closest('details, .code-tabs'))
    const key = location.pathname + '|' + hs.map((h) => h.id).join(',')
    if (box && box.dataset.key === key) return
    if (hs.filter((h) => h.tagName === 'H2').length < 3) {
      box?.remove()
      spyItems = []
      return
    }
    if (!box) {
      box = document.createElement('aside')
      box.id = 'page-outline'
      box.className = 'docs-outline'
      box.setAttribute('aria-label', 'On this page')
      main.prepend(box)
    }
    box.dataset.key = key
    const details = document.createElement('details')
    details.open = wide.matches
    const summary = document.createElement('summary')
    summary.textContent = 'On this page'
    details.append(summary)
    const list = document.createElement('ol')
    spyItems = []
    let parent = null
    for (const h of hs) {
      const li = document.createElement('li')
      const a = document.createElement('a')
      a.href = '#' + h.id
      a.textContent = h.textContent.replace(/#\s*$/, '').trim()
      a.target = '_self'
      li.append(a)
      const it = { h, a, li, parent: null }
      if (h.tagName === 'H2' || !parent) {
        it.sub = document.createElement('ol')
        li.append(it.sub)
        list.append(li)
        parent = it
      } else {
        it.parent = parent
        parent.sub.append(li)
      }
      spyItems.push(it)
    }
    if (hs.length > 30) {
      const input = document.createElement('input')
      input.type = 'search'
      input.placeholder = 'Filter this page'
      input.setAttribute('aria-label', 'Filter this page')
      input.addEventListener('input', () => {
        const q = input.value.trim().toLowerCase()
        box.classList.toggle('filtering', q !== '')
        for (const it of spyItems) it.li.hidden = q !== '' && !it.a.textContent.toLowerCase().includes(q)
        if (q) for (const it of spyItems) if (!it.li.hidden && it.parent) it.parent.li.hidden = false
      })
      details.append(input)
    }
    details.append(list)
    box.replaceChildren(details)
    spy()
  }
  window.addEventListener('scroll', () => requestAnimationFrame(spy), { passive: true })
  wide.addEventListener('change', () => {
    const d = document.querySelector('#page-outline details')
    if (d) d.open = wide.matches
  })

  const load = () => {
    markCurrent()
    groupCode()
    highlight()
    outline()
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
    if (/^\/docs\/examples(\/|$)/.test(path)) return 'examples'
    if (/^\/docs\/reference\/javascript(\/|$)/.test(path)) return 'javascript'
    if (/^\/docs\/reference\/go(\/|$)/.test(path)) return 'go'
    if (/^\/docs\/rest(\/|$)/.test(path)) return 'rest'
    if (/^\/docs\/starters(\/|$)/.test(path)) return 'starters'
    return ''
  }
  // how deep a nav is: the main nav, a section (Contracts, Extensions, Example apps, the API references), an extension's own docs
  const sections = new Set(['contracts', 'extensions', 'examples', 'javascript', 'go', 'rest', 'starters'])
  const depth = (nav) => (nav === '' ? 0 : sections.has(nav) ? 1 : 2)
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

  // the URL changes on a click or Back before the new content arrives, so mark the link right away
  document.addEventListener('click', () => setTimeout(markCurrent, 0))
  window.addEventListener('popstate', markCurrent)

  let timer
  new MutationObserver(() => {
    clearTimeout(timer)
    timer = setTimeout(load, 50)
  }).observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-giscus-term', 'data-title'] })
  load()
})()
