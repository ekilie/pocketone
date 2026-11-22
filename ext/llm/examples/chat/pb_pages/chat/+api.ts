// POST /chat {chat?, question}: the answer as server-sent events, in a stored chat ($llm.chat; a new one when chat is
// empty), armed with the preamble and matching context: "text" events as the model writes, then "done" with the chat's
// id, the sources and timing (or "error").
// GET /chat: the model and what the app knows, for the page's status line.
export function get(ctx: any) {
  const llm = ctx.resolve('llm')
  const kb = llm.knowledge(ctx)
  return ctx.res.json({ model: llm.MODEL, files: kb.files, sections: kb.sections.length })
}

export async function post(ctx: any) {
  const { chat, question } = ctx.req.body()
  if (typeof question !== 'string' || !question.trim()) throw ctx.httpError(400, 'send {chat?, question}')
  const stream = ctx.res.sse()
  try {
    // awaited: the model works off the app's JS VM, so other requests are served while this one streams
    const r = await ctx.resolve('llm').answer(ctx, {
      chatId: chat,
      question,
      onText: (text: string) => stream.send('text', JSON.stringify(text)),
    })
    stream.send('done', JSON.stringify(r))
  } catch (err: any) {
    // not_configured without a provider, unknown_model for a model name that matches none, provider for the
    // provider's own errors (a wrong key, no credits)
    stream.send('error', JSON.stringify(String(err.message || err)))
  }
  stream.close()
}
