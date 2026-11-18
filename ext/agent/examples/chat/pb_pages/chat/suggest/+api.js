// POST /chat/suggest {question, answer}: three follow-up questions, as JSON matching a schema.
module.exports = {
  post(ctx) {
    const { question, answer } = ctx.req.body()
    try {
      return ctx.res.json({ questions: ctx.resolve('agent').followUps(ctx, question || '', answer || '') })
    } catch (err) {
      return ctx.res.json(503, { error: String(err.message || err) })
    }
  },
}
