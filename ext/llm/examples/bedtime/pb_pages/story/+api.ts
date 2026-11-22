// POST /story {prompt, minutes?, pictures?}: saves a new story record and starts making it, without waiting: making one takes a few
// minutes, and a request may wait 30 seconds at most. The page then polls the record (GET
// /api/collections/stories/records/<id>, open to anyone) until its status is done or failed.
export async function post(ctx: any) {
  const story = ctx.resolve('story')
  const { prompt, minutes, pictures } = ctx.req.body()
  if (typeof prompt !== 'string' || !prompt.trim()) throw ctx.httpError(400, 'send {prompt}')
  const busy = story.canStart(ctx.app)
  if (busy) return ctx.res.json(429, { message: busy })

  const r = new Record(ctx.app.findCollectionByNameOrId('stories'))
  r.set('prompt', prompt.trim().slice(0, 500))
  const whole = (v: any, def: number, max: number) => Math.max(1, Math.min(max, Math.round(Number(v) || def)))
  r.set('minutes', whole(minutes, story.DEFAULTS.minutes, story.MAX.minutes))
  r.set('picture_count', whole(pictures, story.DEFAULTS.pictures, story.MAX.pictures))
  r.set('status', 'queued')
  r.set('progress', 'Getting ready')
  ctx.app.save(r)

  // not awaited: the story is made after this request has answered. make() saves its own failures on the record.
  story.make(ctx.app, r.id)
  return ctx.res.json(200, { id: r.id })
}
