// POST /chat {messages: [{role, content}, ...]}: the answer, armed with the preamble and matching context.
// GET /chat: the model's status and what the app knows, for the page's status line.
module.exports = {
  get(ctx) {
    const kb = ctx.resolve('agent').knowledge(ctx)
    return ctx.res.json({ agent: $agent.status(), files: kb.files, sections: kb.sections.length })
  },
  post(ctx) {
    const { messages } = ctx.req.body()
    if (!Array.isArray(messages) || !messages.length) throw ctx.httpError(400, 'send {messages: [{role, content}, ...]}')
    try {
      return ctx.res.json(ctx.resolve('agent').answer(ctx, messages))
    } catch (err) {
      // not_ready while the model downloads, not_configured without a model or API
      return ctx.res.json(503, { error: String(err.message || err) })
    }
  },
}
