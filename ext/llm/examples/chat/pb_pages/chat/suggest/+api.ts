// POST /chat/suggest {question, answer}: three follow-up questions, as JSON matching a schema.
export async function post(ctx: any) {
  const { question, answer } = ctx.req.body()
  try {
    return ctx.res.json({ questions: await ctx.resolve('llm').followUps(ctx, question || '', answer || '') })
  } catch (err: any) {
    return ctx.res.json(503, { error: String(err.message || err) })
  }
}
