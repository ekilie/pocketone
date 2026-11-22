package llm

// TypeScript implements the jsvm types hook: $llm's declarations, over the llm.* contracts' own types.
func (*Extension) TypeScript() string { return typeScript }

const typeScript = `/** A generated or sent file: data is base64. $llm.file(f) makes it a PocketBase file for record.set. */
type LlmFile = { name?: string; type?: string; data?: string; url?: string }

type LlmUsage = { inputTokens?: number; outputTokens?: number; cachedTokens?: number; reasoningTokens?: number; seconds?: number; /** what the call cost, in millionths of a US dollar (1000000 = $1); 0 when free or not known */ costMicroUSD: number; costEstimated?: boolean }

type LlmModels = {
  models: { id: string; provider: string; type: string; name?: string; kinds: string[]; inputs?: string[]; reasoning?: boolean;
    contextTokens?: number; prices?: { unit: string; usd: number }[]; description?: string }[]
}

type LlmJob = { id: string; status: "queued" | "running" | "done" | "failed"; progress?: number; files?: LlmFile[]; outputs?: number; error?: string; model: string; usage: LlmUsage }

type LlmMessage = { role: "system" | "user" | "assistant"; content: string }

type LlmTextInput = {
  /** "provider/model", e.g. "vultr/glm-5.3". */
  model: string
  system?: string
  prompt?: string
  messages?: LlmMessage[]
  /** A JSON schema the answer must match; the parsed answer is then in data. */
  schema?: object | string
  maxTokens?: number
  temperature?: number
  stop?: string[]
  /** How much a reasoning model thinks first: "" (its default), "off", "low", "medium", "high". */
  reasoning?: "" | "off" | "low" | "medium" | "high"
  /** The provider's own fields, passed through. */
  options?: Record<string, any>
}

type LlmTextAnswer = { text: string; data?: any; reasoning?: string; stopReason?: string; model: string; usage: LlmUsage }

type LlmModelsOpts = { provider?: string }

/**
 * Cloud models (the llm.* contracts): each call names provider/model; providers and keys are set in Settings → LLM.
 * Every call that reaches a provider is async (its name ends in Async) and runs off the app's VM, so other requests keep
 * being served while a model works: await it in an async handler.
 */
declare var $llm: {
  text: {
    completeAsync(opts: LlmTextInput): Promise<LlmTextAnswer>
    /** Like completeAsync, calling onText with each piece of the answer as it's written (false stops it). */
    streamAsync(opts: LlmTextInput, onText?: (text: string) => boolean | void, onReasoning?: (text: string) => boolean | void): Promise<LlmTextAnswer>
    /** About how many tokens a text or a conversation is (four characters each); local, not a provider call. */
    count(textOrMessages: string | LlmMessage[] | { text?: string; system?: string; messages?: LlmMessage[] }): number
    modelsAsync(opts?: LlmModelsOpts): Promise<LlmModels>
  }
  image: {
    generateAsync(opts: { model: string; prompt: string; n?: number; size?: string; aspectRatio?: string; options?: Record<string, any> }): Promise<{ files: LlmFile[]; model: string; usage: LlmUsage }>
    editAsync(opts: { model: string; prompt: string; images: LlmFile[]; n?: number; size?: string; aspectRatio?: string; options?: Record<string, any> }): Promise<{ files: LlmFile[]; model: string; usage: LlmUsage }>
    modelsAsync(opts?: LlmModelsOpts): Promise<LlmModels>
  }
  speech: {
    speakAsync(opts: { model: string; text: string; voice?: string; format?: string; speed?: number; instructions?: string; options?: Record<string, any> }): Promise<{ file: LlmFile; model: string; usage: LlmUsage }>
    voicesAsync(model: string): Promise<{ id: string; name?: string; language?: string; gender?: string }[]>
    modelsAsync(opts?: LlmModelsOpts): Promise<LlmModels>
  }
  transcribe: {
    transcribeAsync(opts: { model: string; audio: LlmFile; language?: string; prompt?: string; options?: Record<string, any> }): Promise<{ text: string; language?: string; model: string; usage: LlmUsage }>
    modelsAsync(opts?: LlmModelsOpts): Promise<LlmModels>
  }
  /** Forced alignment: when each line and word of a known text is said in its audio (captions, karaoke). */
  align: {
    alignAsync(opts: { model: string; audio: LlmFile; text: string; mode?: "speech" | "lyrics"; language?: string; options?: Record<string, any> }): Promise<{
      duration: number
      lines: { index: number; text: string; start: number; end: number; score: number; certain: boolean; tag?: string }[]
      words: { line: number; text: string; start: number; end: number; score: number }[]
      unaligned?: { line: number; text: string; reason: string }[]
      model: string
      usage: LlmUsage
    }>
    modelsAsync(opts?: LlmModelsOpts): Promise<LlmModels>
  }
  /** Music is a job: createAsync with wait ("5m") resolves once it's done, files included; without, follow it with getAsync. */
  music: {
    createAsync(opts: { model: string; prompt: string; lyrics?: string; seconds?: number; wait?: string; options?: Record<string, any> }): Promise<LlmJob>
    getAsync(id: string): Promise<LlmJob>
    contentAsync(id: string, index?: number): Promise<LlmFile>
    modelsAsync(opts?: LlmModelsOpts): Promise<LlmModels>
  }
  video: {
    createAsync(opts: { model: string; prompt: string; image?: LlmFile; seconds?: number; size?: string; aspectRatio?: string; wait?: string; options?: Record<string, any> }): Promise<LlmJob>
    getAsync(id: string): Promise<LlmJob>
    contentAsync(id: string, index?: number): Promise<LlmFile>
    modelsAsync(opts?: LlmModelsOpts): Promise<LlmModels>
  }
  embed: {
    embedAsync(opts: { model: string; input: string[]; dimensions?: number; options?: Record<string, any> }): Promise<{ vectors: number[][]; model: string; usage: LlmUsage }>
    modelsAsync(opts?: LlmModelsOpts): Promise<LlmModels>
  }
  /** A generated file (with its bytes) as a PocketBase file: record.set("pictures+", $llm.file(r.files[0])). */
  file(f: LlmFile): any
  /** The same, downloading a file the provider gave only a URL for. */
  fileAsync(f: LlmFile): Promise<any>
  /** Stored conversations (llm.chat/v1, the llm-chat extension). Only sendAsync reaches a provider. */
  chat: {
    /** Starts a chat and returns its id. */
    new(opts?: { title?: string; meta?: Record<string, any> }): string
    /** A stored chat by id, with the options every send uses. */
    open(id: string, opts?: LlmChatOptions): LlmChat
    /** Chats, most recently updated first; meta keeps those whose meta has each key with that value. */
    list(opts?: { meta?: Record<string, any>; limit?: number; offset?: number }): LlmChatInfo[]
    /** Deletes every chat whose meta matches; returns how many. */
    forget(opts: { meta: Record<string, any> }): number
  }
}

type LlmChatOptions = {
  /** "provider/model": every send needs one, here or on the send. */
  model?: string
  reasoning?: "" | "off" | "low" | "medium" | "high"
  system?: string
  /** Sent after the question (history shows the question alone): text, or a function of the question. */
  context?: string | ((question: string) => string)
  /** Messages sent before the summary takes over (default 12). */
  window?: number
  /** Fold the oldest messages into the summary once this many are past the window (default 6). */
  summarizeEvery?: number
  maxTokens?: number
  temperature?: number
}

type LlmChatInfo = { id: string; title: string; summary: string; meta: any; messages: number; created: string; updated: string }

type LlmChat = {
  id: string
  /** Asks and saves both turns. With onText, streams the answer to it as it's written. */
  sendAsync(text: string, opts?: LlmChatOptions & { onText?: (text: string) => boolean | void; onReasoning?: (text: string) => boolean | void; schema?: object | string }): Promise<LlmTextAnswer>
  get(): LlmChatInfo
  history(opts?: { limit?: number; before?: string }): { id: string; role: "user" | "assistant"; content: string; created: string }[]
  summary(): string
  title(text?: string): string
  meta(obj?: Record<string, any>): any
  forget(): void
}
`
