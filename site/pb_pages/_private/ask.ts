// The header's ask box: a question in, a short answer out, citing the docs pages it came from. Retrieval is an LLM
// Search bucket ("docs") holding this site's own docs folder (addFolder: read again every few minutes, each result
// linked to its page and section); the answer is one $llm call per question, counted for the visitor ($llm.for), so
// the guest limits in Settings → LLM apply.

/**
 * The model that answers, as provider/model (a provider added in Settings → LLM, then its model). Set ASK_MODEL to
 * change it without editing this file.
 */
export const MODEL: string = process.env.ASK_MODEL || 'vultr/deepseek-v4-flash'

const BUCKET = 'docs'
const PB_REFERENCE = ['reference/javascript/pocketbase', 'reference/javascript/pocketbase.md', 'reference/go/pocketbase', 'reference/go/pocketbase.md']
const MAX_QUESTION = 300
const PASSAGES = 6
const CACHE_FOR = 60 * 60 * 1000 // an answer is reused for an hour (docs questions repeat a lot)
const MAX_CACHED = 500

const SYSTEM = `You answer questions about PocketOne (PocketBase plus extensions) on its docs site, pocketone.dev.
Answer only from the numbered documentation passages you're given. Cite the passages you use as [1], [2] right after
the sentence they support. Be brief: a few sentences or a short list, with a short code sample when it helps. If the
passages don't answer the question, say so in one sentence. Never write URLs or Markdown links: the citations are the
links.`

/**
 * What code to show, by the reader's choice on the docs' TS | Go tabs. Most people extend PocketOne with TypeScript
 * page code and hooks, and the docs show both, so without this the model often picks the Go sample.
 */
const LANG = {
  ts: `The reader writes TypeScript: page code in pb_pages and hooks in pb_hooks, on the stock pocketone binary. Show
TypeScript (the $-globals such as $llm, $app and ctx), never Go, unless the question asks about Go or the passages only
cover it in Go; then say it's done in Go.`,
  go: `The reader writes Go, building their own binary with PocketOne's extensions. Show Go samples, unless the question
asks about TypeScript, page code or hooks.`,
}

/** Code fences in the other language, which a passage loses before the model sees it (the reader still gets the page). */
const OTHER = {
  ts: /^(go|golang)$/i,
  go: /^(ts|typescript|js|javascript|ets|ejs|tsx|jsx)$/i,
}

/** A passage's text without code blocks in the other language, so the model can't copy a sample the reader won't use. */
function forLang(text: string, lang: 'ts' | 'go'): string {
  return text
    .replace(/^([ \t]*)(```|~~~)[ \t]*([\w+-]*)[^\n]*\n[\s\S]*?^\1\2[ \t]*$/gm, (block, _i, _f, info) =>
      OTHER[lang].test(info) ? '' : block)
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

type Passage = { n: number; title: string; heading: string; url: string; text: string }
type Answer = { sources: Passage[]; text: string; model?: string; at: number }

let ready = false
const cache = new Map<string, Answer>()

/**
 * Makes the docs bucket and points it at the site's docs folder (relative to the app's folder, the one holding
 * pb_data and pb_pages), once per start. LLM Search reads it in the background and again every few minutes.
 */
function ensureDocs() {
  if (ready) return
  $llm.search.ensure({ name: BUCKET, title: 'pocketone.dev docs', owner: 'pocketone.dev',
    description: "Every page under /docs, added by the site's ask box (pb_pages/_private/ask.ts)." })
  // the generated PocketBase API reference (about 1.2 MB) stays out: the guides and the other references link to it
  $llm.search.addFolder({ bucket: BUCKET, path: 'pb_pages/(main)/docs', include: ['*.md'], exclude: ['_private', ...PB_REFERENCE], url: '/docs/{path}' })
  ready = true
}

/** The question as a cache key: lowercase, punctuation and runs of spaces squeezed. */
const key = (q: string) => q.toLowerCase().replace(/[^\p{L}\p{N}$_.]+/gu, ' ').trim()

/**
 * Answers a question: sends the passages first (onSources), then the answer as it's written (onText). A question asked
 * in the last hour gets the stored answer. Throws the $llm error (a limit, no provider) after the sources are sent, so
 * the reader still gets the pages.
 */
export async function ask(
  ctx: any,
  question: string,
  lang: 'ts' | 'go',
  { onSources, onText }: { onSources: (s: Passage[]) => void; onText: (t: string) => void },
): Promise<{ model?: string; usage?: LlmUsage; cached: boolean }> {
  const q = question.trim().slice(0, MAX_QUESTION)
  ensureDocs()
  const k = `${lang} ${key(q)}`
  const hit = cache.get(k)
  if (hit && Date.now() - hit.at < CACHE_FOR) {
    onSources(hit.sources)
    onText(hit.text)
    return { model: hit.model, cached: true }
  }

  const found = await $llm.search.searchAsync({ buckets: BUCKET, query: q, limit: PASSAGES })
  const passages: Passage[] = found.results.map((r, i) => {
    const [title, ...rest] = (r.heading || '').split(' › ')
    return { n: i + 1, title: title || r.document, heading: rest.at(-1) || '', url: r.url || '', text: r.text }
  })
  onSources(passages)
  if (!passages.length) return { cached: false }

  const excerpts = passages
    .map((p) => `[${p.n}] ${p.title}${p.heading ? ` › ${p.heading}` : ''}\n${forLang(p.text, lang) || '(code samples only)'}`)
    .join('\n\n')
  const res = await $llm.for(ctx).text.streamAsync(
    { model: MODEL, system: `${SYSTEM}

${LANG[lang]}`, prompt: `Documentation passages:\n\n${excerpts}\n\nQuestion: ${q}`, maxTokens: 500,
      temperature: 0.2, reasoning: 'off' },
    (t) => onText(t),
  )
  const text = res.text.trim()
  cache.delete(k)
  if (cache.size >= MAX_CACHED) cache.delete(cache.keys().next().value!)
  cache.set(k, { sources: passages, text, model: res.model, at: Date.now() })
  return { model: res.model, usage: res.usage, cached: false }
}
