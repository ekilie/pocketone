// Site search: MiniSearch over assets/search-index.json, which site/sync builds
// from every docs and blog page (one entry per section). Both load the first
// time the search box is opened (or the button is hovered), so pages that
// never search pay nothing. Open with the header button, "/" or Cmd/Ctrl-K.
;(() => {
  const MINISEARCH = 'https://cdn.jsdelivr.net/npm/minisearch@7.1.2/dist/umd/index.min.js'
  const MAX = 12
  const dialog = document.getElementById('search')
  if (!dialog) return
  const input = dialog.querySelector('input')
  const list = dialog.querySelector('.search-results')
  const status = dialog.querySelector('.search-status')
  let engine = null
  let loading = null
  let hits = []
  let active = -1

  const loadScript = (src) =>
    new Promise((resolve, reject) => {
      const s = document.createElement('script')
      s.src = src
      s.onload = resolve
      s.onerror = () => reject(new Error(`couldn't load ${src}`))
      document.head.append(s)
    })

  const ready = () =>
    (loading ||= Promise.all([
      window.MiniSearch ? null : loadScript(MINISEARCH),
      fetch(dialog.dataset.index).then((r) => {
        if (!r.ok) throw new Error(`search index: HTTP ${r.status}`)
        return r.json()
      }),
    ]).then(([, docs]) => {
      engine = new window.MiniSearch({
        fields: ['title', 'heading', 'text'],
        storeFields: ['url', 'title', 'heading', 'text'],
        searchOptions: { boost: { title: 3, heading: 2 }, prefix: true, fuzzy: 0.2, combineWith: 'AND' },
      })
      engine.addAll(docs)
    }))

  const esc = (s) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c])
  const reEsc = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

  // A short piece of the section's text around the first matched term, with the terms marked.
  const snippet = (text, terms) => {
    if (!text) return ''
    const re = terms.length ? new RegExp(terms.map(reEsc).join('|'), 'gi') : null
    const at = re ? text.search(re) : -1
    const start = Math.max(0, at - 60)
    let s = text.slice(start, start + 180)
    if (start > 0) s = '…' + s.replace(/^\S*\s/, '')
    if (start + 180 < text.length) s = s.replace(/\s\S*$/, '') + '…'
    s = esc(s)
    return re ? s.replace(new RegExp(terms.map((t) => reEsc(esc(t))).join('|'), 'gi'), '<mark>$&</mark>') : s
  }

  const select = (i) => {
    active = i
    list.querySelectorAll('[role=option]').forEach((li, j) => {
      li.setAttribute('aria-selected', String(j === i))
      if (j === i) li.scrollIntoView({ block: 'nearest' })
    })
  }

  const render = () => {
    const q = input.value.trim()
    if (!q) {
      hits = []
      list.replaceChildren()
      status.textContent = ''
      return
    }
    if (!engine) {
      status.textContent = 'Loading…'
      return
    }
    hits = engine.search(q).slice(0, MAX)
    list.innerHTML = hits
      .map(
        (h, i) => `<li role="option" id="search-hit-${i}" aria-selected="false"><a href="${esc(h.url)}">
  <span class="hit-title">${esc(h.title)}${h.heading ? ` <span aria-hidden="true">›</span> ${esc(h.heading)}` : ''}</span>
  <span class="hit-text">${snippet(h.text, h.terms)}</span></a></li>`
      )
      .join('')
    status.textContent = hits.length ? '' : `No results for “${q}”`
    select(hits.length ? 0 : -1)
  }

  const open = () => {
    if (!dialog.open) dialog.showModal()
    input.select()
    ready().then(render, (err) => {
      loading = null
      status.textContent = `Search couldn't load (${err.message}).`
    })
  }

  const go = (i) => {
    const a = list.querySelectorAll('[role=option] a')[i]
    if (!a) return
    dialog.close()
    location.href = a.href
  }

  input.addEventListener('input', render)
  input.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      if (!hits.length) return
      select((active + (e.key === 'ArrowDown' ? 1 : -1) + hits.length) % hits.length)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      go(Math.max(active, 0))
    }
  })
  list.addEventListener('click', (e) => {
    if (e.target.closest('a')) dialog.close()
  })
  // A click on the backdrop (the dialog element itself, outside its box) closes it.
  dialog.addEventListener('click', (e) => {
    if (e.target === dialog) dialog.close()
  })

  document.querySelectorAll('[data-search-open]').forEach((b) => {
    b.addEventListener('click', open)
    b.addEventListener('pointerenter', () => ready().catch(() => (loading = null)), { once: true })
  })
  document.addEventListener('keydown', (e) => {
    if (dialog.open) return
    const typing = e.target.closest?.('input, textarea, select, [contenteditable]')
    if ((e.key === 'k' && (e.metaKey || e.ctrlKey)) || (e.key === '/' && !typing && !e.metaKey && !e.ctrlKey && !e.altKey)) {
      e.preventDefault()
      open()
    }
  })
})()
