// Arms $llm with what this app knows: a preamble (preamble.md) and context files (context/*.md), read from disk. For
// each question it picks the few sections of the context that match best and hands only those to the model, so every
// question is small and cheap instead of sending every file every time.

/**
 * The model that answers, as provider/model: a provider you added in Settings → LLM, then that provider's model. Set
 * CHAT_MODEL to change it without editing this file (CHAT_MODEL=or/openai/gpt-4o-mini pocketone serve).
 */
export const MODEL: string = process.env.CHAT_MODEL || 'vultr/glm-5.2'

type Section = { file: string; source: string; title: string; text: string; terms?: { [w: string]: number }; length?: number }
type Knowledge = { preamble: string; files: string[]; sections: Section[]; df: { [w: string]: number }; avgLength: number }

const privateDir = (ctx: any): string => `${ctx.pagesDir}/_private`
const read = (path: string): string => toString($os.readFile(path))

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

/** Splits one context file into sections at its ## and ### headings (never inside ``` blocks). */
function sections(name: string, text: string): Section[] {
  const source = (text.match(/^Source: (\S+)/m) || [])[1] || ''
  const title = (text.match(/^# (.+)$/m) || [])[1] || name
  const out: Section[] = []
  let heading = ''
  let buf: string[] = []
  let fenced = false
  const flush = () => {
    const body = buf.join('\n').trim()
    buf = []
    if (body) out.push({ file: name, source, title: heading ? `${title} › ${heading}` : title, text: body.slice(0, 1800) })
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

/**
 * What an answer cost, in US dollars: the provider's own figure when it reports one (OpenRouter does on every answer),
 * else an estimate from the model's listed prices and the tokens used (exact: false). Undefined when neither is known.
 */
type Cost = { usd: number; exact: boolean } | undefined

const prices: { [model: string]: { input: number; output: number } | null } = {} // per token, by provider/model

/** The model's per-token input and output prices, from its provider's model list (read once per model). */
async function priceOf(model: string) {
  if (model in prices) return prices[model]
  const provider = model.split('/')[0]
  const list = await $llm.text.modelsAsync({ provider })
  const m = list.models.find((x) => x.id === model)
  let input = 0, output = 0
  for (const p of m?.prices || []) {
    const unit = p.unit.toLowerCase()
    const perToken = /\b(1m|m)\b/.test(unit) ? p.usd / 1e6 : p.usd
    if (unit.includes('cached')) continue
    if (/input|prompt/.test(unit)) input = perToken
    else if (/output|completion/.test(unit)) output = perToken
  }
  prices[model] = input || output ? { input, output } : null
  return prices[model]
}

/** The answer's cost: the provider's, or estimated from the listed prices. */
async function costOf(model: string, usage: LlmUsage): Promise<Cost> {
  if (usage.costUSD != null) return { usd: usage.costUSD, exact: true }
  try {
    const p = await priceOf(model)
    if (!p) return undefined
    const cached = usage.cachedTokens || 0
    return { usd: ((usage.inputTokens || 0) - cached) * p.input + (usage.outputTokens || 0) * p.output, exact: false }
  } catch {
    return undefined
  }
}

let cached: Knowledge | undefined // a module loads once per app (per runtime in the pool), so this lasts across requests

/**
 * What the app knows, read from disk once. In dev mode it's read again on every call, so edits to preamble.md or a
 * context file show up at once.
 */
export function knowledge(ctx: any): Knowledge {
  if (cached && !ctx.dev) return cached
  const dir = `${privateDir(ctx)}/context`
  const files = $os
    .readDir(dir)
    .filter((e) => !e.isDir() && e.name().endsWith('.md'))
    .map((e) => e.name())
  const all = files.flatMap((name) => sections(name, read(`${dir}/${name}`)))
  const df: { [w: string]: number } = {}
  for (const s of all) {
    s.terms = {}
    for (const w of words(`${s.title} ${s.title} ${s.title} ${s.text}`)) s.terms[w] = (s.terms[w] || 0) + 1 // titles count thrice
    s.length = Object.values(s.terms).reduce((a, b) => a + b, 0)
    for (const w in s.terms) df[w] = (df[w] || 0) + 1
  }
  const kb: Knowledge = {
    preamble: read(`${privateDir(ctx)}/preamble.md`).trim(),
    files,
    sections: all,
    df,
    avgLength: all.reduce((a, s) => a + (s.length || 0), 0) / Math.max(1, all.length),
  }
  cached = kb
  return kb
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
  const kb = knowledge(ctx)
  const id = chatId || $llm.chat.new({ title: question.slice(0, 80) })
  // the question, plus the one before it, so a follow-up ("and how do I undo that?") still finds its topic
  const before = chatId ? $llm.chat.open(id).history({ limit: 2 }).filter((m) => m.role === 'user').map((m) => m.content) : []
  const context = search(kb, [...before, question].join(' '), 3)
  const excerpts = context.map((s, i) => `[${i + 1}] ${s.title}\n${s.text}`).join('\n\n')
  // The chat sends the preamble, a summary of older turns, the turns since, then this question with its excerpts after
  // it, and keeps the question as sent, so each turn's prompt starts exactly like the last one did (providers bill a
  // repeated start cheaper). Reasoning is off: the excerpts hold the answer, and thinking would only cost time.
  const chat = $llm.chat.open(id, { model: MODEL, system: kb.preamble, maxTokens: 600, temperature: 0.3, reasoning: 'off' })
  const started = Date.now()
  const res = await chat.sendAsync(question, { context: `Documentation excerpts to answer from:\n\n${excerpts}`, onText })
  return {
    chat: id,
    answer: res.text.trim(),
    model: res.model,
    usage: res.usage,
    cost: await costOf(res.model, res.usage),
    ms: Date.now() - started,
    armedWith: ['preamble.md', ...new Set(context.map((s) => s.file))],
    sources: context.map((s) => ({ title: s.title, url: s.source })),
  }
}

/** Three short follow-up questions, as JSON in the shape asked (a schema). */
export async function followUps(ctx: any, question: string, reply: string): Promise<string[]> {
  const res: LlmTextAnswer = await $llm.text.completeAsync({
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

