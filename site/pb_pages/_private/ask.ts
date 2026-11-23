// The header's ask box: a question in, a short answer out, citing the docs pages it came from. Retrieval is an LLM
// Search bucket ("docs") filled from this site's own pages; the answer is one $llm call per question, counted for the
// visitor ($llm.for), so the guest limits in Settings → LLM apply.

/**
 * The model that answers, as provider/model (a provider added in Settings → LLM, then its model). Set ASK_MODEL to
 * change it without editing this file.
 */
export const MODEL: string = process.env.ASK_MODEL || 'vultr/deepseek-v4-flash'

/** Where a question goes on for a conversation. */
export const CHAT_URL = 'https://learn.pocketone.dev'

const BUCKET = 'docs'
const DOCS = '(main)/docs' // under pb_pages
const MAX_QUESTION = 300
const PASSAGES = 6
const SYNC_EVERY = 10 * 60 * 1000 // a deploy changes pages without a restart: check them this often
const MAX_CACHED = 500

const SYSTEM = `You answer questions about PocketOne (PocketBase plus extensions) on its docs site, pocketone.dev.
Answer only from the numbered documentation passages you're given. Cite the passages you use as [1], [2] right after
the sentence they support. Be brief: a few sentences or a short list, with a short code sample when it helps. If the
passages don't answer the question, say so in one sentence. Never write URLs or Markdown links: the citations are the
links.`

type Passage = { n: number; title: string; heading: string; url: string; text: string }
type Answer = { sources: Passage[]; text: string; model?: string }

let syncedAt = 0
let syncing: Promise<void> | undefined
let stamp = '' // changes whenever the docs do, so cached answers go with them
const cache = new Map<string, Answer>()

const read = (path: string): string => toString($os.readFile(path))
const breathe = () => sleep.async(0)

/** Every docs page as [name, path]: name is its path under (main), "docs/extensions/llm.md". */
function pages(root: string, rel = 'docs'): [string, string][] {
  const out: [string, string][] = []
  for (const e of $os.readDir(`${root}/${rel}`)) {
    const n = e.name()
    if (n.startsWith('_') || n.startsWith('+') || n.startsWith('.')) continue
    if (e.isDir()) out.push(...pages(root, `${rel}/${n}`))
    else if (n.endsWith('.md')) out.push([`${rel}/${n}`, `${root}/${rel}/${n}`])
  }
  return out
}

/** A page's URL from its name: docs/a/b.md → /docs/a/b, docs/a/index.md → /docs/a. */
const pageURL = (name: string): string => '/' + name.replace(/\.md$/, '').replace(/\/index$/, '')

/**
 * Keeps the docs bucket in step with the pages: adds a page whose text changed (by its hash, so an unchanged page costs
 * nothing) and removes pages that are gone. LLM Search embeds the new chunks in the background.
 */
export async function syncDocs(ctx: any): Promise<void> {
  if (Date.now() - syncedAt < SYNC_EVERY) return
  syncing ??= (async () => {
    const t0 = Date.now()
    $llm.search.ensure({ name: BUCKET, title: 'pocketone.dev docs', owner: 'pocketone.dev',
      description: "Every page under /docs, added by the site's ask box (pb_pages/_private/ask.ts). Don't edit by hand." })
    const have = new Map($llm.search.sources(BUCKET).map((s) => [s.name, s.hash]))
    const root = `${ctx.pagesDir}/(main)`
    const list = pages(root)
    let added = 0
    const hashes: string[] = []
    for (const [i, [name, path]] of list.entries()) {
      if (i % 20 === 19) await breathe()
      const text = read(path)
      const hash = $security.sha256(text)
      hashes.push(hash)
      if (have.get(name) !== hash) {
        $llm.search.add({ bucket: BUCKET, name, text })
        added++
      }
      have.delete(name)
    }
    if (have.size) $llm.search.remove(BUCKET, [...have.keys()])
    const next = $security.sha256(hashes.join(''))
    if (next !== stamp) cache.clear()
    stamp = next
    syncedAt = Date.now()
    if (added || have.size) console.log(`ask: docs bucket: ${added} pages added, ${have.size} removed (${list.length} pages, ${Date.now() - t0} ms)`)
  })().finally(() => (syncing = undefined))
  return syncing
}

/**
 * Goldmark's automatic heading ID (as site/sync did for the old search index): ASCII letters, digits and _ lowercased,
 * runs of spaces and - become one -, the rest drops; a repeat on the page gets -1, -2, ...
 */
function headingID(text: string, seen: Set<string>): string {
  let id = ''
  for (const c of text) {
    if (/[A-Za-z0-9_]/.test(c)) id += c.toLowerCase()
    else if ((c === ' ' || c === '\t' || c === '-') && !id.endsWith('-')) id += '-'
  }
  id ||= 'heading'
  let out = id
  for (let i = 1; seen.has(out); i++) out = `${id}-${i}`
  seen.add(out)
  return out
}

/** A heading's text as goldmark sees it for the ID: links and images as their text, no code or emphasis marks. */
const headingText = (s: string): string =>
  s.replace(/!?\[([^\]]*)\]\([^)]*\)/g, '$1').replace(/<[^>]+>/g, '').replace(/`|\*\*|__/g, '').trim()

/** The anchor of the section holding line (1-based) of a page: its nearest H2-H6 above, or "" for the top. */
function anchorAt(path: string, line: number): string {
  let text: string
  try {
    text = read(path)
  } catch {
    return ''
  }
  const seen = new Set<string>()
  let anchor = ''
  let fence = ''
  const lines = text.split('\n')
  let i = 0
  if (lines[0]?.trim() === '---') for (i = 1; i < lines.length && !['---', '...'].includes(lines[i].trim()); i++);
  for (; i < Math.min(line, lines.length); i++) {
    const t = lines[i].trim()
    if (fence) {
      if (t.startsWith(fence)) fence = ''
      continue
    }
    if (t.startsWith('```') || t.startsWith('~~~')) {
      fence = t.slice(0, 3)
      continue
    }
    const m = lines[i].match(/^(#{1,6})\s+(.+?)\s*#*\s*$/)
    if (!m) continue
    const id = headingID(headingText(m[2]), seen)
    if (m[1].length > 1) anchor = id
  }
  return anchor
}

/** The question as a cache key: lowercase, punctuation and runs of spaces squeezed. */
const key = (q: string) => q.toLowerCase().replace(/[^\p{L}\p{N}$_.]+/gu, ' ').trim()

/**
 * Answers a question: sends the passages first (onSources), then the answer as it's written (onText). A question asked
 * before (since the docs last changed) gets the stored answer. Throws the $llm error (a limit, no provider) after the
 * sources are sent, so the reader still gets the pages.
 */
export async function ask(
  ctx: any,
  question: string,
  { onSources, onText }: { onSources: (s: Passage[]) => void; onText: (t: string) => void },
): Promise<{ model?: string; usage?: LlmUsage; cached: boolean }> {
  const q = question.trim().slice(0, MAX_QUESTION)
  await syncDocs(ctx)
  const hit = cache.get(key(q))
  if (hit) {
    onSources(hit.sources)
    onText(hit.text)
    return { model: hit.model, cached: true }
  }

  const root = `${ctx.pagesDir}/(main)`
  const found = await $llm.search.searchAsync({ buckets: BUCKET, query: q, limit: PASSAGES })
  const passages: Passage[] = found.results.map((r, i) => {
    const [title, ...rest] = (r.heading || '').split(' › ')
    const anchor = anchorAt(`${root}/${r.document}`, r.startLine)
    return { n: i + 1, title: title || pageURL(r.document), heading: rest.at(-1) || '', url: pageURL(r.document) + (anchor ? `#${anchor}` : ''), text: r.text }
  })
  onSources(passages)
  if (!passages.length) return { cached: false }

  const excerpts = passages.map((p) => `[${p.n}] ${p.title}${p.heading ? ` › ${p.heading}` : ''}\n${p.text}`).join('\n\n')
  const res = await $llm.for(ctx).text.streamAsync(
    { model: MODEL, system: SYSTEM, prompt: `Documentation passages:\n\n${excerpts}\n\nQuestion: ${q}`, maxTokens: 500,
      temperature: 0.2, reasoning: 'off' },
    (t) => onText(t),
  )
  const text = res.text.trim()
  if (cache.size >= MAX_CACHED) cache.delete(cache.keys().next().value!)
  cache.set(key(q), { sources: passages, text, model: res.model })
  return { model: res.model, usage: res.usage, cached: false }
}
