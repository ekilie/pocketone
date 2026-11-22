// POST /inspire {avoid?}: one topic for a new story, unlike the stories already told (the model sees their titles and topics).
export async function post(ctx: any) {
  const { avoid } = ctx.req.body() || {}
  const earlier = Array.isArray(avoid) ? avoid.filter((a: any) => typeof a === 'string').slice(-20).map((a: string) => a.slice(0, 300)) : []
  const idea = await ctx.resolve('story').inspire(ctx.app, earlier) // avoid: what this page suggested before
  return ctx.res.json(200, { idea })
}
