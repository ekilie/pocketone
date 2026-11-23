// Arms $llm with what this app knows: a preamble (preamble.md) and two sets of docs. PocketOne's user docs come from
// pocketone.dev itself (its llms.txt and llms-full.txt); PocketBase's own docs come from the Markdown copy PocketOne
// keeps of pocketbase.io/docs (agent/skills/pocketone/reference/pocketbase/llms-full.txt, built from the PocketBase
// site). For each question it picks the few sections that match best, from either set, labels each with the docs it
// came from, and hands only those to the model, so every question is small and cheap.

/**
 * The model that answers, as provider/model: a provider you added in Settings → LLM, then that provider's model. Set
 * CHAT_MODEL to change it without editing this file (CHAT_MODEL=or/openai/gpt-4o-mini pocketone serve).
 */
export const MODEL: string = process.env.CHAT_MODEL || 'vultr/glm-5.3'

/**
 * The site to answer from: its llms.txt lists the docs by section, its llms-full.txt has every page's text. Set
 * DOCS_URL to answer from another copy (DOCS_URL=http://127.0.0.1:8090 for a site you're running).
 */
export const DOCS_URL: string = (process.env.DOCS_URL || 'https://pocketone.dev').replace(/\/$/, '')

/**
 * The user docs: the pages llms.txt lists under these sections, plus the pages below each of them (an extension's
 * guides). The rest of the site (the generated reference under Reference, and Extend PocketOne) is for people writing
 * extensions or looking up exact signatures, so the chat leaves it out.
 */
const SECTIONS = ['Get started', 'Database', 'Users and sign-in', 'Hooks and routes', 'Websites', 'Email', 'AI', 'Go',
  'Deploy and run', 'How-to guides', 'REST API']
/** User docs listed under other sections. */
const ALSO: string[] = []

/**
 * PocketBase's docs as one file (pages separated by "---", each starting "Source: <url>"): a URL, or a path to a local
 * copy. Set PB_DOCS to use another (a checkout's agent/skills/pocketone/reference/pocketbase/llms-full.txt, say).
 */
export const PB_DOCS: string = process.env.PB_DOCS ||
  'https://raw.githubusercontent.com/pocketonehq/pocketone/main/agent/skills/pocketone/reference/pocketbase/llms-full.txt'

type Docs = 'PocketOne' | 'PocketBase'
type Section = { docs: Docs; file: string; source: string; title: string; text: string; terms?: { [w: string]: number }; length?: number }
type Knowledge = {
  preamble: string
  files: string[] // PocketOne pages
  pbFiles: string[] // PocketBase pages
  pbError?: string // PocketBase's docs didn't load: the chat answers from PocketOne's alone
  sections: Section[]
  df: { [w: string]: number }
  avgLength: number
  at: number
}

const privateDir = (ctx: any): string => `${ctx.pagesDir}/_private`
const read = (path: string): string => toString($os.readFile(path))

/** A URL's path: https://pocketone.dev/docs/x → /docs/x (the engine has no URL class). */
const pathOf = (url: string): string => url.replace(/^[a-z]+:\/\/[^/]+/i, '') || '/'

/** Fetches a text file off the app's JS VM. */
async function fetchText(url: string): Promise<string> {
  const res = await $http.send.async({ url, method: 'GET', timeout: 30 })
  if (res.statusCode !== 200) throw new Error(`${url}: HTTP ${res.statusCode}`)
  return toString(res.body)
}

/** The user docs' URLs (paths), from llms.txt's sections. */
function userPages(index: string): string[] {
  const out: string[] = [...ALSO]
  let keep = false
  for (const line of index.split('\n')) {
    const h = line.match(/^## (.+)$/)
    if (h) keep = SECTIONS.includes(h[1].trim())
    const link = keep && line.match(/^- \[[^\]]*\]\((\S+?)\)/)
    if (link) out.push(pathOf(link[1]))
  }
  return out
}

/** Whether a page is user docs: one listed, or below one (not its extension reference page). */
const isUserPage = (path: string, pages: string[]) =>
  !path.endsWith('/extension') && pages.some((p) => path === p || path.startsWith(p + '/'))

const STOP = new Set(
  `a an and are as at be but by can do does for from has have how i if in into is it its me my of on or so that the
   their them then there these this to was we what when where which who why will with you your about any all also not
   no yes up out use using get set one our us should would could just like more than very`.split(/\s+/)
)

// The docs' words for what people ask about ("instances" where people say "apps", "host" for "server").
const SYNONYMS: { [w: string]: string[] } = { app: ['instance'], server: ['host'], site: ['instance'], backup: ['restore', 'pitr'],
  undo: ['restore'], email: ['mail'], ai: ['mcp', 'llm'] }

/** Search terms: lowercase words without stop words, single letters or a plural "s". */
const words = (s: string): string[] =>
  s
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .filter((w) => w.length > 1 && !STOP.has(w))
    .map((w) => (w.length > 3 && w.endsWith('s') && !w.endsWith('ss') ? w.slice(0, -1) : w))

/** A question's terms, with the docs' words for them added. */
const queryWords = (s: string): string[] => words(s).flatMap((w) => [w, ...(SYNONYMS[w] || [])])

/** Splits one page into sections at its ## and ### headings (never inside ``` blocks). */
function sections(docs: Docs, name: string, text: string): Section[] {
  const source = (text.match(/^Source: (\S+)/m) || [])[1] || ''
  const title = (text.match(/^# (.+)$/m) || [])[1] || name
  const out: Section[] = []
  let heading = ''
  let buf: string[] = []
  let fenced = false
  const flush = () => {
    const body = buf.join('\n').trim()
    buf = []
    if (body) out.push({ docs, file: name, source, title: heading ? `${title} › ${heading}` : title, text: body.slice(0, 1800) })
  }
  for (const line of text.split('\n')) {
    if (line.trim().startsWith('```')) fenced = !fenced
    if (!fenced && /^#{2,3} /.test(line)) {
      flush()
      heading = line.replace(/^#+ /, '').trim()
    } else if (!fenced && (/^# /.test(line) || line.startsWith('Source: '))) {
      // the page title and source are kept on every section instead
    } else {
      buf.push(line)
    }
  }
  flush()
  return out
}

let cached: Knowledge | undefined // a module loads once per app, so this lasts across requests
let loading: Promise<Knowledge> | undefined
const MAX_AGE = 10 * 60 * 1000 // fetched again after 10 minutes, so a docs update shows up

/** What the app knows, if it's loaded (for the page's header; answer() loads it). */
export const knowledgeNow = (): Knowledge | undefined => cached

/** What the app knows: the preamble, and the site's user docs split into sections and indexed for search. */
export async function knowledge(ctx: any): Promise<Knowledge> {
  if (cached && Date.now() - cached.at < MAX_AGE) return cached
  loading ??= load(ctx).finally(() => (loading = undefined))
  return loading
}

/** PocketBase's docs: fetched (off the VM) from a URL, or read from a local file. */
async function readPocketBase(): Promise<string> {
  return /^https?:/.test(PB_DOCS) ? fetchText(PB_DOCS) : read(PB_DOCS)
}

// The engine gives each task (a request, a promise continuation) a few seconds of JS time, and splitting and indexing
// both sets of docs takes longer: yield now and then, so each stretch is a short task of its own.
const breathe = () => sleep.async(0)

async function load(ctx: any): Promise<Knowledge> {
  const t0 = Date.now()
  const [index, full, pb] = await Promise.all([
    fetchText(`${DOCS_URL}/llms.txt`),
    fetchText(`${DOCS_URL}/llms-full.txt`),
    readPocketBase().catch((err) => err as Error),
  ])
  const pages = userPages(index)
  const files: string[] = []
  const pbFiles: string[] = []
  const all: Section[] = []
  // both files: pages separated by "---", each starting "Source: <url>"
  let n = 0
  for (const page of full.split(/\n---\n/)) {
    if (++n % 25 === 0) await breathe()
    const source = (page.match(/^Source: (\S+)/m) || [])[1]
    if (!source) continue
    const path = pathOf(source)
    if (!isUserPage(path, pages)) continue
    files.push(path)
    all.push(...sections('PocketOne', path, page))
  }
  if (typeof pb === 'string') {
    for (const page of pb.split(/\n---\n/)) {
      if (++n % 25 === 0) await breathe()
      const source = (page.match(/^Source: (\S+)/m) || [])[1]
      if (!source) continue
      const path = pathOf(source)
      pbFiles.push(path)
      all.push(...sections('PocketBase', path, page))
    }
  }
  const df: { [w: string]: number } = {}
  for (const [i, s] of all.entries()) {
    if (i % 200 === 199) await breathe()
    s.terms = {}
    for (const w of words(`${s.title} ${s.title} ${s.title} ${s.text}`)) s.terms[w] = (s.terms[w] || 0) + 1 // titles count thrice
    s.length = Object.values(s.terms).reduce((a, b) => a + b, 0)
    for (const w in s.terms) df[w] = (df[w] || 0) + 1
  }
  console.log(`docs: ${files.length} PocketOne pages, ${pbFiles.length} PocketBase pages, ${all.length} sections in ${Date.now() - t0} ms`)
  cached = {
    preamble: read(`${privateDir(ctx)}/preamble.md`).trim(),
    files,
    pbFiles,
    pbError: typeof pb === 'string' ? undefined : String(pb.message || pb),
    sections: all,
    df,
    avgLength: all.reduce((a, s) => a + (s.length || 0), 0) / Math.max(1, all.length),
    at: Date.now(),
  }
  return cached
}

/** The n sections that best match query (BM25). */
function search(kb: Knowledge, query: string, n: number): Section[] {
  const q = queryWords(query)
  const N = kb.sections.length
  return kb.sections
    .map((s) => {
      let score = 0
      for (const w of q) {
        const tf = s.terms![w] || 0
        if (!tf) continue
        const idf = Math.log(1 + (N - kb.df[w] + 0.5) / (kb.df[w] + 0.5))
        score += (idf * tf * 2.2) / (tf + 1.2 * (0.25 + (0.75 * s.length!) / kb.avgLength))
      }
      return { s, score }
    })
    .filter((x) => x.score > 0)
    .sort((a, b) => b.score - a.score)
    .slice(0, n)
    .map((x) => x.s)
}

/**
 * Answers a question in a stored chat ($llm.chat, the llm-chat extension), armed with the preamble and the
 * best-matching context. The chat keeps the conversation: the page only holds its id. A new chat starts when chatId is
 * empty. onText gets the answer as it's written.
 */
export async function answer(
  ctx: any,
  { chatId, question, onText }: { chatId?: string; question: string; onText?: (t: string) => void },
) {
  const kb = await knowledge(ctx)
  // $llm.for(ctx): every call counts for this visitor (their guest cookie), so the per-guest and all-guests limits in
  // Settings → LLM apply; guest calls are refused until an all-guests limit is set there
  const llm = $llm.for(ctx)
  const id = chatId || llm.chat.new({ title: question.slice(0, 80) })
  // the question, plus the one before it, so a follow-up ("and how do I undo that?") still finds its topic
  const before = chatId ? llm.chat.open(id).history({ limit: 2 }).filter((m) => m.role === 'user').map((m) => m.content) : []
  const context = search(kb, [...before, question].join(' '), 3)
  const excerpts = context.map((s, i) => `[${i + 1}] ${s.docs} docs: ${s.title} (${s.source})\n${s.text}`).join('\n\n')
  // The chat sends the preamble, a summary of older turns, the turns since, then this question with its excerpts after
  // it, and keeps the question as sent, so each turn's prompt starts exactly like the last one did (providers bill a
  // repeated start cheaper). Reasoning is off: the excerpts hold the answer, and thinking would only cost time.
  const chat = llm.chat.open(id, { model: MODEL, system: kb.preamble, maxTokens: 600, temperature: 0.3, reasoning: 'off' })
  const started = Date.now()
  const res = await chat.sendAsync(question, { context: `Documentation excerpts to answer from:\n\n${excerpts}`, onText })
  return {
    chat: id,
    answer: res.text.trim(),
    model: res.model,
    usage: res.usage,
    ms: Date.now() - started,
    armedWith: ['preamble.md', ...new Set(context.map((s) => `${s.docs}: ${s.file}`))],
    sources: context.map((s) => ({ title: `${s.docs} › ${s.title}`, url: s.source })),
  }
}

/** Three short follow-up questions, as JSON in the shape asked (a schema). */
export async function followUps(ctx: any, question: string, reply: string): Promise<string[]> {
  const res: LlmTextAnswer = await $llm.for(ctx).text.completeAsync({
    model: MODEL,
    reasoning: 'off',
    system: 'Suggest three short follow-up questions a PocketOne user might ask next. Each under eight words.',
    prompt: `Question: ${question}\nAnswer: ${reply}`,
    // an object at the top: providers' strict JSON modes want one
    schema: {
      type: 'object',
      properties: { questions: { type: 'array', items: { type: 'string' }, minItems: 3, maxItems: 3 } },
      required: ['questions'],
      additionalProperties: false,
    },
    maxTokens: 200,
  })
  return (res.data?.questions || []).slice(0, 3)
}

