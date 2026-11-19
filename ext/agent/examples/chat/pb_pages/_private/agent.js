// Arms $agent with what this app knows: a preamble (preamble.md) and context files (context/*.md), read from disk.
// For each question it picks the few sections of the context that match best and hands only those to the model, so a
// small model on a CPU answers in seconds instead of reading every file every time.

const privateDir = (ctx) => `${ctx.pagesDir}/_private`
const read = (path) => toString($os.readFile(path))

const STOP = new Set(
  `a an and are as at be but by can do does for from has have how i if in into is it its me my of on or so that the
   their them then there these this to was we what when where which who why will with you your about any all also not
   no yes up out use using get set one our us should would could just like more than very`.split(/\s+/)
)

// The docs' words for what people ask about ("instances" where people say "apps", "host" for "server").
const SYNONYMS = { app: ['instance'], server: ['host'], site: ['instance'], backup: ['restore', 'pitr'],
  undo: ['restore'], email: ['mail'], ai: ['mcp', 'agent'] }

/** Search terms: lowercase words without stop words, single letters or a plural "s". */
const words = (s) =>
  s
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .filter((w) => w.length > 1 && !STOP.has(w))
    .map((w) => (w.length > 3 && w.endsWith('s') && !w.endsWith('ss') ? w.slice(0, -1) : w))

/** A question's terms, with the docs' words for them added. */
const queryWords = (s) => words(s).flatMap((w) => [w, ...(SYNONYMS[w] || [])])

/** Splits one context file into sections at its ## and ### headings (never inside ``` blocks). */
function sections(name, text) {
  const source = (text.match(/^Source: (\S+)/m) || [])[1] || ''
  const title = (text.match(/^# (.+)$/m) || [])[1] || name
  const out = []
  let heading = ''
  let buf = []
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

let cached // a module loads once per app (per runtime in the pool), so this lasts across requests

/**
 * What the app knows, read from disk once. In dev mode it's read again on every call, so edits to preamble.md or a
 * context file show up at once.
 */
function knowledge(ctx) {
  if (cached && !ctx.dev) return cached
  const dir = `${privateDir(ctx)}/context`
  const files = $os
    .readDir(dir)
    .filter((e) => !e.isDir() && e.name().endsWith('.md'))
    .map((e) => e.name())
  const all = files.flatMap((name) => sections(name, read(`${dir}/${name}`)))
  const df = {}
  for (const s of all) {
    s.terms = {}
    for (const w of words(`${s.title} ${s.title} ${s.title} ${s.text}`)) s.terms[w] = (s.terms[w] || 0) + 1 // titles count thrice
    s.length = Object.values(s.terms).reduce((a, b) => a + b, 0)
    for (const w in s.terms) df[w] = (df[w] || 0) + 1
  }
  const kb = {
    preamble: read(`${privateDir(ctx)}/preamble.md`).trim(),
    files,
    sections: all,
    df,
    avgLength: all.reduce((a, s) => a + s.length, 0) / Math.max(1, all.length),
  }
  cached = kb
  return kb
}

/** The n sections that best match query (BM25). */
function search(kb, query, n) {
  const q = queryWords(query)
  const N = kb.sections.length
  return kb.sections
    .map((s) => {
      let score = 0
      for (const w of q) {
        const tf = s.terms[w] || 0
        if (!tf) continue
        const idf = Math.log(1 + (N - kb.df[w] + 0.5) / (kb.df[w] + 0.5))
        score += (idf * tf * 2.2) / (tf + 1.2 * (0.25 + (0.75 * s.length) / kb.avgLength))
      }
      return { s, score }
    })
    .filter((x) => x.score > 0)
    .sort((a, b) => b.score - a.score)
    .slice(0, n)
    .map((x) => x.s)
}

/**
 * Answers the conversation's last question, armed with the preamble and the best-matching context. onText, if given,
 * gets the answer as it's written ($agent.stream); without it the answer comes back whole ($agent.complete).
 */
function answer(ctx, messages, onText) {
  const kb = knowledge(ctx)
  const recent = messages.slice(-6) // the last few turns are enough; every token is read on a CPU
  // the question, plus the one before it, so a follow-up ("and how do I undo that?") still finds its topic
  const query = recent
    .filter((m) => m.role === 'user')
    .slice(-2)
    .map((m) => m.content)
    .join(' ')
  const context = search(kb, query, 3)
  // The preamble and the conversation so far never change between turns, so they go first, and the excerpts (which
  // change with every question) ride along with the new question. A prompt that starts the same as the last one is
  // read only from where it differs: the model keeps the start in its prompt cache.
  const excerpts = context.map((s, i) => `[${i + 1}] ${s.title}\n${s.text}`).join('\n\n')
  const last = recent[recent.length - 1]
  const turns = [
    ...recent.slice(0, -1),
    // the question first: next turn, this message is history with the question alone, and the shared start runs to
    // the end of the question
    { role: 'user', content: `${last.content}\n\nDocumentation excerpts to answer from:\n\n${excerpts}` },
  ]
  const started = Date.now()
  const opts = { system: kb.preamble, messages: turns, maxTokens: 350, temperature: 0.3 }
  const res = onText ? $agent.stream(opts, onText) : $agent.complete(opts)
  return {
    answer: res.text.trim(),
    model: res.model,
    backend: res.backend,
    usage: res.usage,
    ms: Date.now() - started,
    armedWith: ['preamble.md', ...new Set(context.map((s) => s.file))],
    sources: context.map((s) => ({ title: s.title, url: s.source })),
  }
}

/** Three short follow-up questions, as JSON the model can only write in the shape asked (a schema). */
function followUps(ctx, question, reply) {
  const res = $agent.complete({
    system: 'Suggest three short follow-up questions a PocketOne user might ask next. Each under eight words.',
    prompt: `Question: ${question}\nAnswer: ${reply}`,
    schema: { type: 'array', items: { type: 'string' }, minItems: 3, maxItems: 3 },
    maxTokens: 60,
  })
  return res.data
}

module.exports = { knowledge, answer, followUps }
