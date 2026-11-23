// Answers a question from the docs in pb_pages/docs: LLM Search finds the passages that match, and one model call writes
// a short answer citing them. The call goes through $llm.for(ctx), so it counts for the visitor (their account, or a
// guest cookie) and the limits in Settings → LLM apply.

/** The model that answers, as provider/model from Settings → LLM. ASK_MODEL overrides it. */
const MODEL: string = process.env.ASK_MODEL || 'vultr/deepseek-v4-flash'
const BUCKET = 'docs'
const PASSAGES = 5

const SYSTEM = `You answer questions on a documentation site. Answer only from the numbered passages you're given, and
cite the ones you use as [1], [2] after the sentence they support. Be brief. If the passages don't answer the question,
say so in one sentence.`

type Source = { n: number; title: string; heading: string; url: string }

let ready = false

/** Makes the docs bucket and points it at pb_pages/docs, once per start; LLM Search reads it again as files change. */
function ensureDocs() {
  if (ready) return
  $llm.search.ensure({ name: BUCKET, title: 'Docs', description: 'The pages in pb_pages/docs, for the ask page.' })
  $llm.search.addFolder({ bucket: BUCKET, path: 'pb_pages/docs', include: ['*.md'], url: '/docs/{path}' })
  ready = true
}

/**
 * The answer to a question, with the pages it drew on. Anything that stops it (no provider or embedding model yet, a
 * limit reached) comes back as error, written for the reader, so the page shows it instead of failing.
 */
export async function answer(ctx: any, question: string): Promise<{ text: string; sources: Source[]; error?: string }> {
  let sources: Source[] = []
  try {
    ensureDocs()
    const found = await $llm.search.searchAsync({ buckets: BUCKET, query: question.slice(0, 300), limit: PASSAGES })
    sources = found.results.map((r, i) => {
      const [title, ...rest] = (r.heading || '').split(' › ')
      return { n: i + 1, title: title || r.document, heading: rest.at(-1) || '', url: r.url || '' }
    })
    if (!sources.length) return { text: '', sources }

    const passages = found.results.map((r, i) => `[${i + 1}] ${r.heading || r.document}\n${r.text}`).join('\n\n')
    const res = await $llm.for(ctx).text.completeAsync({
      model: MODEL,
      system: SYSTEM,
      prompt: `Passages:\n\n${passages}\n\nQuestion: ${question}`,
      maxTokens: 400,
      temperature: 0.2,
    })
    return { text: res.text.trim(), sources }
  } catch (err: any) {
    return { text: '', sources, error: err.message }
  }
}
