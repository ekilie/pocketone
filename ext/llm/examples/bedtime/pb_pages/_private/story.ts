// Makes a bedtime story for adults (one topic, told in depth) from a prompt, in the background, saving each step on its `stories` record so the page can
// show progress (it polls the record) and play the story once it's done.
//
//   1. write: the story, one sentence per line
//   2. read it aloud: one voiceover of the whole story
//   3. align: when each sentence and word is said in the voiceover ($llm.align)
//   4. place the pictures: the model reads the timed transcript and picks where each picture begins and what it shows,
//      plus a prompt for the song
//   5. paint the pictures (all at once), and 6. compose the song, at the same time
//
// Every $llm call is awaited, so the work runs off the app's JS VM: the app keeps serving pages (and this story's own
// progress polls) while the models work. The request that starts a story doesn't wait for it; see story/+api.ts.

/** The models, as provider/model (a provider you added in Settings → LLM, then its model). Each can be set from the environment. */
export const MODELS = {
  text: process.env.STORY_TEXT_MODEL || 'vultr/mimo-v2.6-flash', // fast, and writes without thinking first
  image: process.env.STORY_IMAGE_MODEL || 'vultr/z-image-turbo',
  speech: process.env.STORY_SPEECH_MODEL || 'vultr/auk',
  // for a story in a language the speech model doesn't speak (auk speaks English and Chinese)
  speechOther: process.env.STORY_SPEECH_MODEL_OTHER || 'vultr/chatterbox',
  align: process.env.STORY_ALIGN_MODEL || 'vultr/aligner-1',
  music: process.env.STORY_MUSIC_MODEL || 'vultr/ace-step-1.5',
  // suggests topics for Inspire me
  idea: process.env.STORY_IDEA_MODEL || process.env.STORY_TEXT_MODEL || 'vultr/mimo-v2.6-flash',
}

/** A story's length in minutes, and how many pictures it gets, when the form doesn't say. */
export const DEFAULTS = { minutes: 1, pictures: 3 }
/** The most either may be. */
export const MAX = { minutes: 5, pictures: 5 }
/** About how many words a narrator reads in a minute at a bedtime pace. */
const WORDS_PER_MINUTE = 140

/** How many stories may be in the making at once, and how many may start in an hour (a 5-minute story with 5 pictures costs about $0.15 on Vultr). */
export const LIMITS = {
  making: Number(process.env.STORY_MAX_MAKING || 3),
  perHour: Number(process.env.STORY_MAX_PER_HOUR || 20),
}

/** The languages a speech model speaks, where it's known (GET /v1/audio/voices?model= on Vultr lists them). */
const SPEAKS: { [model: string]: string[] } = { 'vultr/auk': ['en', 'zh'] }

/** What the front page sends when the prompt is left empty: the model picks the topic. */
export const ANY_TOPIC = 'Your pick'

const WRITE = (minutes: number) => `You write bedtime stories for adults: calm, unhurried narration to fall asleep to,
read aloud in about ${minutes} minute${minutes === 1 ? '' : 's'}: ${Math.round(minutes * WORDS_PER_MINUTE * 0.9)} to ${Math.round(minutes * WORDS_PER_MINUTE * 1.1)} words.
Each story picks one topic and goes deep: the universe, consciousness and the mind, the history of an idea, an event,
a place or a person, how something in nature works. Choose a narrow, specific angle within the request rather than a
survey, and carry the listener through it with concrete detail: names, places, dates, numbers, how things felt and
looked. Stay accurate: tell only what is known, and say so where something is uncertain or disputed; never invent
facts, quotes or sources. The tone is warm, curious and quiet, never dramatic or sensational; long, flowing sentences
are welcome, and the pace slows toward the end, closing on a gentle, spacious thought.
If the request is "${ANY_TOPIC}", choose a topic yourself, something with depth that a curious adult may not know well.
Write the story in the language the request is written in, unless the request asks for another language; "language"
is that language's ISO 639-1 code ("en", "es", "ja"). "topic" is the story's subject in a short phrase. Give the story
one sentence per item of "sentences", in order: each item is a single sentence of at most about 30 words, shown as a
caption while it's read, never a paragraph. "look" describes, for an illustrator, what recurs through the story's
pictures (a place, a person with their dress and era, an object), so every picture can show it the same way.`

const WRITE_SCHEMA = {
  type: 'object',
  properties: {
    title: { type: 'string' },
    language: { type: 'string' },
    topic: { type: 'string' },
    look: { type: 'string' },
    sentences: { type: 'array', items: { type: 'string' } },
  },
  required: ['title', 'language', 'topic', 'look', 'sentences'],
}

const PLACE = (n: number) => `You illustrate a story that has been read aloud. You get its transcript, one
sentence per line, each with the second it starts. Choose exactly ${n} pictures, spread through the story: for each, the
line where it should appear (the first picture at line 0, then in order, each where the story moves somewhere new or
something new happens) and a prompt for an illustrator describing what the picture shows. Describe anything that recurs
(a place, a person, an object) the same way in every prompt. No text, letters or diagrams in pictures.
"style" is one line describing the look shared by every picture (for example: muted oil painting, soft
light, deep blues and ambers), suited to the topic and calm. "music" is a prompt for quiet instrumental ambient music that suits the story: instruments, mood and tempo, no
drums.`

const PLACE_SCHEMA = {
  type: 'object',
  properties: {
    style: { type: 'string' },
    music: { type: 'string' },
    pictures: {
      type: 'array',
      items: {
        type: 'object',
        properties: { line: { type: 'integer' }, prompt: { type: 'string' } },
        required: ['line', 'prompt'],
      },
    },
  },
  required: ['style', 'music', 'pictures'],
}

type Line = { index: number; text: string; start: number; end: number }
type Word = { line: number; text: string; start: number; end: number }
export type Picture = { start: number; prompt: string; image?: string }
export type Story = {
  title: string
  language: string // ISO 639-1
  topic: string
  look: string
  sentences: string[]
  // from here on, filled in as the story is made
  duration?: number // seconds of narration
  lines?: Line[] // when each sentence is said
  words?: Word[] // when each word is said
  style?: string
  music?: string
  pictures?: Picture[] // in order; each shows from its start to the next one's
}

/** Whether another story may start now; a reason when not. */
/** The most the app spends on models in a day (UTC), in US dollars. Past it, nothing new is made until tomorrow. */
export const DAILY_BUDGET = Number(process.env.STORY_DAILY_BUDGET || 1)

export const OUT_OF_TOKENS = 'Out of tokens for today; try again tomorrow. Please enjoy something from our archives.'
export const RESTING = 'The storyteller can\'t reach its models right now. Please enjoy something from our archives.'

/** What a call cost, in millionths of a dollar (usage.costMicroUSD, 0 when not known). */
const micro = (u: any) => (u && typeof u.costMicroUSD === 'number' ? u.costMicroUSD : 0)

/** Saves what a model call cost (a costs row) and returns it in dollars. */
function record(app: any, kind: string, result: any, story = ''): number {
  const m = micro(result && result.usage)
  try {
    const r = new Record(app.findCollectionByNameOrId('costs'))
    r.set('kind', kind)
    r.set('model', String((result && result.model) || ''))
    r.set('micro_usd', m)
    r.set('story', story)
    app.save(r)
  } catch (err: any) {
    console.log(`bedtime: couldn't record a cost: ${err && err.message ? err.message : err}`)
  }
  return m / 1e6
}

/** What the app has spent on models today, since 00:00 UTC. */
export function spentToday(app: any): number {
  const since = new Date().toISOString().slice(0, 10) + ' 00:00:00.000Z'
  const rows = app.findRecordsByFilter('costs', 'created >= {:since}', '', 100000, 0, { since })
  return rows.reduce((t: number, r: any) => t + r.getInt('micro_usd'), 0) / 1e6
}

let healthCache: { at: number; problem: string | null } | null = null

/**
 * Whether every model the app uses is there: $llm exists, each model's provider is in Settings → LLM and answers, and
 * offers the model (Vultr lists its aligner only for a working key, so a wrong key shows up here too). null when all
 * is well, else what's wrong. Checked at most once a minute.
 */
export async function health(): Promise<string | null> {
  if (healthCache && Date.now() - healthCache.at < 60_000) return healthCache.problem
  let problem: string | null = null
  if (typeof $llm === 'undefined') {
    problem = 'this PocketOne has no $llm (the llm extension)'
  } else {
    const needs: [string, string][] = [['text', MODELS.text], ['speech', MODELS.speech], ['align', MODELS.align],
      ['image', MODELS.image], ['music', MODELS.music]]
    for (const [kind, model] of needs) {
      try {
        const provider = model.split('/')[0]
        const list = await ($llm as any)[kind].modelsAsync({ provider })
        if (!(list.models || []).some((m: any) => m.id === model)) {
          problem = `${model} isn't offered: is a provider named "${provider}" in Settings → LLM, with a working key?`
          break
        }
      } catch (err: any) {
        problem = `${model}: ${err && err.message ? err.message : err}`
        break
      }
    }
  }
  healthCache = { at: Date.now(), problem }
  if (problem) console.log(`bedtime: not healthy: ${problem}`)
  return problem
}

/**
 * Why nothing new may be made right now, or null. message is for the listener; reason says more, for the app's admins.
 * Checks the models (health), the day's budget, then the limits on stories in the making and per hour.
 */
export async function refusal(app: any, what: 'story' | 'idea'): Promise<{ status: number; message: string; reason?: string } | null> {
  const problem = await health()
  if (problem) return { status: 503, message: RESTING, reason: problem }
  const spent = spentToday(app)
  if (spent >= DAILY_BUDGET) return { status: 429, message: OUT_OF_TOKENS, reason: `spent $${spent.toFixed(2)} of $${DAILY_BUDGET} today` }
  if (what === 'idea') return null
  const busy = canStart(app)
  return busy ? { status: 429, message: busy } : null
}

/** Whether another story may start now, by the limits on stories in the making and per hour; a reason when not. */
export function canStart(app: any): string | null {
  const making = app.countRecords('stories', $dbx.exp("status IN ('queued', 'writing', 'making')"))
  if (making >= LIMITS.making) return `${making} stories are being made right now; try again in a few minutes.`
  const since = new Date(Date.now() - 3600_000).toISOString().replace('T', ' ')
  const recent = app.countRecords('stories', $dbx.exp('created > {:since}', { since }))
  if (recent >= LIMITS.perHour) return 'That\'s a lot of stories for one hour; try again later.'
  return null
}

/** Makes the story for record id. Never throws: a failure is saved on the record. */
export async function make(app: any, id: string): Promise<void> {
  const save = (fields: { [k: string]: any }) => {
    const r = app.findRecordById('stories', id)
    for (const k in fields) r.set(k, fields[k])
    app.save(r)
  }
  let spent = 0
  // a JSON answer from the text model; a model sometimes writes broken JSON (bad_answer), and a second try usually works
  const ask = async (kind: string, opts: any) => {
    const once = () => $llm.text.completeAsync({ model: MODELS.text, reasoning: 'off', maxTokens: 12000, ...opts })
    const r = await once().catch((err: any) => {
      if (!String(err && err.message).startsWith('bad_answer')) throw err
      return once()
    })
    spent += record(app, kind, r, id)
    return r.data
  }
  try {
    const rec = app.findRecordById('stories', id)
    const prompt = rec.getString('prompt')
    const minutes = rec.getInt('minutes') || DEFAULTS.minutes
    const wanted = rec.getInt('picture_count') || DEFAULTS.pictures

    // 1. write
    save({ status: 'writing', progress: 'Writing the story' })
    const written = await ask('write', { system: WRITE(minutes), prompt: `Tonight's story: ${prompt}`, schema: WRITE_SCHEMA, temperature: 0.9 })
    const story: Story = {
      title: String(written.title || 'A bedtime story'),
      language: String(written.language || 'en').toLowerCase().slice(0, 2),
      topic: String(written.topic || ''),
      look: String(written.look || ''),
      sentences: (written.sentences || []).map((s: any) => String(s).replace(/\s+/g, ' ').trim()).filter(Boolean),
    }
    if (story.sentences.length < 5) throw new Error(`the story came back too short: ${story.sentences.length} sentences, ${JSON.stringify(written).slice(0, 300)}`)
    save({ status: 'making', title: story.title, story, progress: 'Reading it aloud', cost: spent })

    // 2. one voiceover of the whole story
    // the speech model for the story's language, told the language (chatterbox picks its checkpoint by it)
    const speech = SPEAKS[MODELS.speech]?.includes(story.language) === false ? MODELS.speechOther : MODELS.speech
    const voice = await $llm.speech.speakAsync({ model: speech, text: story.sentences.join(' '),
      options: { language: story.language } })
    spent += record(app, 'speech', voice, id)
    save({ progress: 'Listening for when each word is said', cost: spent })

    // 3. align: one line per sentence
    const a = await $llm.align.alignAsync({ model: MODELS.align, audio: voice.file, text: story.sentences.join('\n'), mode: 'speech',
      language: story.language })
    spent += record(app, 'align', a, id)
    story.duration = a.duration
    story.lines = a.lines.map((l: any) => ({ index: l.index, text: l.text, start: l.start, end: l.end }))
    story.words = a.words.map((x: any) => ({ line: x.line, text: x.text, start: x.start, end: x.end }))
    if (!story.lines.length) throw new Error('the voiceover could not be lined up with the story')
    save({ story, progress: 'Choosing the pictures', cost: spent })

    // 4. place the pictures from the timed transcript
    const n = wanted
    const minGap = Math.min(10, a.duration / n / 2) // seconds a picture shows at least
    const transcript = story.lines.map((l) => `[${l.index}] ${l.start.toFixed(1)}s ${l.text}`).join('\n')
    const placed = await ask('place', {
      system: PLACE(n),
      prompt: `Topic: ${story.topic}\nWhat recurs in the pictures: ${story.look}\nLength: ${a.duration.toFixed(0)} seconds\n\n${transcript}`,
      schema: PLACE_SCHEMA,
    })
    const startOf = new Map(story.lines.map((l) => [l.index, l.start]))
    const chosen = (placed.pictures || []).filter((p: any) => p && p.prompt).slice(0, n)
    if (!chosen.length) throw new Error('no pictures were chosen')
    // where the model put them, when every choice is a real line, in order, and not too close to the one before;
    // otherwise spread them evenly, each at the line nearest its share of the story
    let starts = chosen.map((p: any, i: number) => (i === 0 ? 0 : startOf.get(Number(p.line))))
    const placedWell = starts.every((t: any, i: number) => t !== undefined && (i === 0 || t >= starts[i - 1] + minGap))
    if (!placedWell) {
      starts = chosen.map((_: any, i: number) => {
        if (i === 0) return 0
        const target = (a.duration * i) / chosen.length
        return story.lines!.reduce((best, l) => (Math.abs(l.start - target) < Math.abs(best - target) ? l.start : best), 0)
      })
    }
    const pictures: Picture[] = chosen.map((p: any, i: number) => ({ start: starts[i] as number, prompt: String(p.prompt) }))
    story.style = String(placed.style || 'soft watercolor, warm night light')
    story.music = String(placed.music || 'quiet ambient music on soft piano and strings')
    story.pictures = pictures
    save({ story, cost: spent })

    // 5. paint the pictures and 6. compose the song, at the same time; progress saved as each piece arrives
    const done = { pictures: 0, music: false }
    const report = () =>
      save({ progress: `Painting pictures ${done.pictures}/${pictures.length} · ${done.music ? 'Song ready' : 'Composing the song'}` })
    report()
    const painted = Promise.all(pictures.map(async (p) => {
      const r = await $llm.image.generateAsync({ model: MODELS.image, prompt: `${story.style}. ${p.prompt}. No text.`, aspectRatio: '16:9' })
      spent += record(app, 'image', r, id)
      done.pictures++
      report()
      return r.files[0]
    }))
    const composed = (async () => {
      const r = await $llm.music.createAsync({
        model: MODELS.music,
        prompt: `${story.music}. Soft ambient, instrumental, slow, spacious, no drums, no vocals.`,
        lyrics: '[instrumental]',
        seconds: 90,
        wait: '6m',
      })
      spent += record(app, 'music', r, id)
      done.music = true
      report()
      return r.files && r.files[0]
    })()
    const [images, song] = await Promise.all([painted, composed])

    // the files, then each picture's file name (the record keeps them in the order they were set)
    const r = app.findRecordById('stories', id)
    r.set('narration', $llm.file(voice.file))
    r.set('pictures', images.map((f: any) => $llm.file(f)))
    if (song) r.set('music', await $llm.fileAsync(song))
    app.save(r)
    const names = app.findRecordById('stories', id).get('pictures')
    pictures.forEach((p, i) => { p.image = names[i] })
    save({ story, status: 'done', progress: '', cost: spent })
  } catch (err: any) {
    const message = String(err && err.message ? err.message : err)
    console.log(`bedtime: story ${id} failed: ${message}`)
    try {
      save({ status: 'failed', error: message, cost: spent })
    } catch (_) {
      // the record is gone
    }
  }
}

const INSPIRE = `You suggest topics for bedtime stories for adults: one topic told slowly and in depth, such as the
universe, consciousness and the mind, a moment or person in history, or how something in nature works. You get the
stories already told. Suggest one new topic in the same spirit that none of them covers, as a short, specific phrase
of a few words to a sentence ("The night the 1859 solar storm lit up the sky"). If none are listed, suggest a topic a
curious grown-up would enjoy falling asleep to.`

/** A topic for a new story, unlike the ones already told and the suggestions in avoid. */
export async function inspire(app: any, avoid: string[] = []): Promise<string> {
  const told = app.findRecordsByFilter('stories', "status = 'done'", '-views,-created', 200, 0)
  const list = told.map((r: any) => {
    const topic = (JSON.parse(r.getString('story') || '{}') || {}).topic
    return `- ${r.getString('title')}${topic ? ` (${topic})` : ''}`
  }).join('\n')
  const r = await $llm.text.completeAsync({
    model: MODELS.idea,
    system: INSPIRE,
    prompt: (list ? `Stories already told:\n${list}` : 'No stories have been told yet.') +
      (avoid.length ? `\n\nAlready suggested, so suggest something different:\n${avoid.map((a) => `- ${a}`).join('\n')}` : ''),
    schema: { type: 'object', properties: { topic: { type: 'string' } }, required: ['topic'] },
    reasoning: 'off',
    maxTokens: 4000,
    temperature: 1,
  })
  record(app, 'inspire', r)
  return String((r.data && r.data.topic) || '').trim()
}

/** A story in the making whose record hasn't moved for this long was cut off (the app restarted). */
export const STALE_MS = 15 * 60_000
