// POST /chat {messages: [{role, content}, ...]}: the answer as server-sent events, armed with the preamble and matching
// context: "text" events as the model writes, then "done" with the sources and timing (or "error").
// GET /chat: the model's status and what the app knows, for the page's status line.
module.exports = {
  get(ctx) {
    const kb = ctx.resolve('agent').knowledge(ctx)
    return ctx.res.json({ agent: $agent.status(), files: kb.files, sections: kb.sections.length })
  },
  post(ctx) {
    const { messages } = ctx.req.body()
    if (!Array.isArray(messages) || !messages.length) throw ctx.httpError(400, 'send {messages: [{role, content}, ...]}')
    const stream = ctx.res.sse()
    try {
      const r = ctx.resolve('agent').answer(ctx, messages, (text) => stream.send('text', JSON.stringify(text)))
      stream.send('done', JSON.stringify(r))
    } catch (err) {
      // not_ready while the model downloads, not_configured without a model or API
      stream.send('error', JSON.stringify(String(err.message || err)))
    }
    stream.close()
  },
}
