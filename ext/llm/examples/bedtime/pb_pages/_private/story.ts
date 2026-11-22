// Makes a bedtime story from a prompt, in the background, saving each step on its `stories` record so the page can
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
  text: process.env.STORY_TEXT_MODEL || 'vultr/glm-5.2',
  image: process.env.STORY_IMAGE_MODEL || 'vultr/z-image-turbo',
  speech: process.env.STORY_SPEECH_MODEL || 'vultr/auk',
  align: process.env.STORY_ALIGN_MODEL || 'vultr/aligner-1',
  music: process.env.STORY_MUSIC_MODEL || 'vultr/ace-step-1.5',
}

/** A story's length in minutes, and how many pictures it gets, when the form doesn't say. */
export const DEFAULTS = { minutes: 1, pictures: 3 }
/** The most either may be. */
export const MAX = { minutes: 10, pictures: 12 }
/** About how many words a narrator reads in a minute at a bedtime pace. */
const WORDS_PER_MINUTE = 140

/** How many stories may be in the making at once, and how many may start in an hour (a 5-minute story with 5 pictures costs about $0.15 on Vultr). */
export const LIMITS = {
  making: Number(process.env.STORY_MAX_MAKING || 3),
  perHour: Number(process.env.STORY_MAX_PER_HOUR || 20),
}

const WRITE = (minutes: number) => `You write gentle bedtime stories for young children, read aloud in about
${minutes} minute${minutes === 1 ? '' : 's'}: ${Math.round(minutes * WORDS_PER_MINUTE * 0.9)} to ${Math.round(minutes * WORDS_PER_MINUTE * 1.1)} words.
Keep it calm and warm: no danger, no villains, nothing scary. The story slows down toward the end and closes with the
main character falling asleep. Use simple words, short sentences, gentle sounds and repetition children like.
Give the story one sentence per item of "sentences", in order. "character" describes the main character for an
illustrator (kind of animal or child, colors, clothes), so every picture can show them the same way.`

const WRITE_SCHEMA = {
  type: 'object',
  properties: {
    title: { type: 'string' },
    character: { type: 'string' },
    sentences: { type: 'array', items: { type: 'string' } },
  },
  required: ['title', 'character', 'sentences'],
}

const PLACE = (n: number) => `You illustrate a bedtime story that has been read aloud. You get its transcript, one
sentence per line, each with the second it starts. Choose exactly ${n} pictures, spread through the story: for each, the
line where it should appear (the first picture at line 0, then in order, each where the story moves somewhere new or
something new happens) and a prompt for an illustrator describing what the picture shows. Describe the main character
the same way in every prompt. No text or letters in pictures.
"style" is one line describing the look shared by every picture (for example: soft watercolor, warm lamplight, deep
blue night). "music" is a prompt for a quiet instrumental lullaby that suits the story: instruments, mood and tempo, no
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
  character: string
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
export function canStart(app: any): string | null {
  const making = app.countRecords('stories', $dbx.exp("status IN ('queued', 'writing', 'making')"))
  if (making >= LIMITS.making) return `${making} stories are being made right now; try again in a few minutes.`
  const since = new Date(Date.now() - 3600_000).toISOString().replace('T', ' ')
  const recent = app.countRecords('stories', $dbx.exp('created > {:since}', { since }))
  if (recent >= LIMITS.perHour) return 'That\'s a lot of stories for one hour; try again later.'
  return null
}

const cost = (u: any) => (u && typeof u.costUSD === 'number' ? u.costUSD : 0)

/** Makes the story for record id. Never throws: a failure is saved on the record. */
export async function make(app: any, id: string): Promise<void> {
  const save = (fields: { [k: string]: any }) => {
    const r = app.findRecordById('stories', id)
    for (const k in fields) r.set(k, fields[k])
    app.save(r)
  }
  let spent = 0
  // a JSON answer from the text model; a model sometimes writes broken JSON (bad_answer), and a second try usually works
  const ask = async (opts: any) => {
    const once = () => $llm.text.completeAsync({ model: MODELS.text, reasoning: 'off', maxTokens: 12000, ...opts })
    const r = await once().catch((err: any) => {
      if (!String(err && err.message).startsWith('bad_answer')) throw err
      return once()
    })
    spent += cost(r.usage)
    return r.data
  }
  try {
    const rec = app.findRecordById('stories', id)
    const prompt = rec.getString('prompt')
    const minutes = rec.getInt('minutes') || DEFAULTS.minutes
    const wanted = rec.getInt('picture_count') || DEFAULTS.pictures

    // 1. write
    save({ status: 'writing', progress: 'Writing the story' })
    const written = await ask({ system: WRITE(minutes), prompt: `Tonight's story: ${prompt}`, schema: WRITE_SCHEMA, temperature: 0.9 })
    const story: Story = {
      title: String(written.title || 'A bedtime story'),
      character: String(written.character || ''),
      sentences: (written.sentences || []).map((s: any) => String(s).replace(/\s+/g, ' ').trim()).filter(Boolean),
    }
    if (story.sentences.length < 5) throw new Error('the story came back too short')
    save({ status: 'making', title: story.title, story, progress: 'Reading it aloud', cost: spent })

    // 2. one voiceover of the whole story
    const voice = await $llm.speech.speakAsync({ model: MODELS.speech, text: story.sentences.join(' ') })
    spent += cost(voice.usage)
    save({ progress: 'Listening for when each word is said', cost: spent })

    // 3. align: one line per sentence
    const a = await $llm.align.alignAsync({ model: MODELS.align, audio: voice.file, text: story.sentences.join('\n'), mode: 'speech' })
    spent += cost(a.usage)
    story.duration = a.duration
    story.lines = a.lines.map((l: any) => ({ index: l.index, text: l.text, start: l.start, end: l.end }))
    story.words = a.words.map((x: any) => ({ line: x.line, text: x.text, start: x.start, end: x.end }))
    if (!story.lines.length) throw new Error('the voiceover could not be lined up with the story')
    save({ story, progress: 'Choosing the pictures', cost: spent })

    // 4. place the pictures from the timed transcript
    const n = wanted
    const minGap = Math.min(10, a.duration / n / 2) // seconds a picture shows at least
    const transcript = story.lines.map((l) => `[${l.index}] ${l.start.toFixed(1)}s ${l.text}`).join('\n')
    const placed = await ask({
      system: PLACE(n),
      prompt: `Main character: ${story.character}\nLength: ${a.duration.toFixed(0)} seconds\n\n${transcript}`,
      schema: PLACE_SCHEMA,
    })
    const startOf = new Map(story.lines.map((l) => [l.index, l.start]))
    const pictures: Picture[] = []
    for (const p of placed.pictures || []) {
      const start = pictures.length === 0 ? 0 : startOf.get(Number(p.line))
      if (start === undefined || !p.prompt) continue
      if (pictures.length && start < pictures[pictures.length - 1].start + minGap) continue // too close to the last one
      pictures.push({ start, prompt: String(p.prompt) })
    }
    if (!pictures.length) throw new Error('no pictures were chosen')
    story.style = String(placed.style || 'soft watercolor, warm night light')
    story.music = String(placed.music || 'a quiet lullaby on soft piano and harp')
    story.pictures = pictures
    save({ story, cost: spent })

    // 5. paint the pictures and 6. compose the song, at the same time; progress saved as each piece arrives
    const done = { pictures: 0, music: false }
    const report = () =>
      save({ progress: `Painting pictures ${done.pictures}/${pictures.length} · ${done.music ? 'Song ready' : 'Composing the song'}` })
    report()
    const painted = Promise.all(pictures.map(async (p) => {
      const r = await $llm.image.generateAsync({ model: MODELS.image, prompt: `${story.style}. ${p.prompt}. No text.`, aspectRatio: '16:9' })
      spent += cost(r.usage)
      done.pictures++
      report()
      return r.files[0]
    }))
    const composed = (async () => {
      const r = await $llm.music.createAsync({
        model: MODELS.music,
        prompt: `${story.music}. Soft ambient lullaby, instrumental, slow, gentle, no drums, no vocals.`,
        lyrics: '[instrumental]',
        seconds: 90,
        wait: '6m',
      })
      spent += cost(r.usage)
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

/** A story in the making whose record hasn't moved for this long was cut off (the app restarted). */
export const STALE_MS = 15 * 60_000
