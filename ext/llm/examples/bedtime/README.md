# Bedtime Story

A PocketPages app with no Go (`pb_pages`, one migration and the stock `pocketone` binary) that makes a bedtime story
from a prompt: read aloud by one narrator, with pictures that slowly pan and zoom, the words lit up as they're said, and
a quiet song underneath. You pick its length in minutes and how many pictures it gets. Each story is saved, so it can
be replayed and shared.

It shows `$llm` doing a job longer than one request:

1. **write** (`$llm.text.completeAsync` with a schema): a title, the main character described for an illustrator, and
   the story one sentence per item;
2. **read it aloud** (`$llm.speech.speakAsync`): one voiceover of the whole story;
3. **align** (`$llm.align.alignAsync`): when each sentence and word is said in the voiceover;
4. **place the pictures** (`completeAsync` again): the model reads the timed transcript and chooses where each picture
   begins and what it shows, plus the look shared by every picture and a prompt for the song;
5. **paint** the pictures (`$llm.image.generateAsync`, all at once) and 6. **compose** the song (`$llm.music.createAsync`
   with `wait`), at the same time.

The pictures stay consistent because every prompt repeats the same character description and style line.

`POST /story` saves a `stories` record and starts the work without awaiting it, then answers at once. The work saves
each step's progress on the record, and the page polls the record through the records API (`listRule` and `viewRule`
are open, so anyone with the link can watch and play) until it's done. Every `$llm` call is awaited, so the models work
off the app's JS VM and the app keeps serving pages meanwhile.

```
pb_migrations/1763800000_stories.js   the stories collection: prompt, length, status, progress, the story with its timings, files
pb_pages/
  index.ets            ask for a story (prompt, minutes, pictures); stories already told
  story/+api.ts        POST /story {prompt, minutes?, pictures?}: saves the record, starts making it
  story/[id].ets       progress while it's made, then the player (scrubber, space to pause, arrows to skip 10 s)
  _private/story.ts    the models, the limits and the six steps
```

## Run it

```bash
cd ext/llm/examples/bedtime
pocketone superuser upsert admin@example.com password --dir=pb_data
pocketone serve --dir=pb_data
```

Add a Vultr provider named `vultr` in Settings → LLM (the dashboard at `/_/`), then open `http://127.0.0.1:8090`. A
1-minute story with 3 pictures takes about two minutes to make and costs about 8 cents on Vultr, most of it the song
and the pictures.

The models are set in [`_private/story.ts`](pb_pages/_private/story.ts), each overridable from the environment:
`STORY_TEXT_MODEL`, `STORY_IMAGE_MODEL`, `STORY_SPEECH_MODEL`, `STORY_ALIGN_MODEL`, `STORY_MUSIC_MODEL`. So are the
limits for a public copy: `STORY_MAX_MAKING` stories in the making at once (3) and `STORY_MAX_PER_HOUR` started an hour
(20).

A story that was being made when the app stopped shows as unfinished after 15 minutes.
