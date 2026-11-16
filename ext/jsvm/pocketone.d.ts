// -------------------------------------------------------------------
// PocketOne (ext/jsvm): what this engine adds to PocketBase's JS API.
// Hand-written; internal/typegen appends it to the generated declarations.
// -------------------------------------------------------------------

/**
 * Every Go function and method has an `.async` twin: it runs off the app's JS loop and returns a promise, so the app
 * serves other requests while it waits.
 *
 * ```js
 * const post = await $app.findRecordById.async("posts", id)
 * await sleep.async(100)
 * ```
 *
 * Only Go functions have it at runtime; on a JS function it's undefined.
 *
 * @group PocketOne
 */
interface Function {
  async<A extends any[], R>(this: (...args: A) => R, ...args: A): Promise<Awaited<R>>;
}

/** @group PocketOne */
interface ImportMeta {
  /** The module's file URL, `file:///.../pb_hooks/main.pb.mts`. */
  url: string;
  /** The module's file path. */
  filename: string;
  /** The module's folder. */
  dirname: string;
}

/** @group PocketOne */
interface Console {
  log(...data: any[]): void;
  error(...data: any[]): void;
  warn(...data: any[]): void;
  info(...data: any[]): void;
  debug(...data: any[]): void;
  trace(...data: any[]): void;
}
/** Writes to the app's log. @group PocketOne */
declare var console: Console;

/**
 * Loads a CommonJS module (`.js`, `.cjs`, `.cts`, `.json`, a `node_modules` package) or an ES module without top-level
 * await (`.mjs`, `.ts`, `.mts`), relative to the calling file. A module loads once per app.
 *
 * @group PocketOne
 */
declare function require(id: string): any;

/** Inside a CommonJS module: the module. @group PocketOne */
declare var module: { exports: any; id: string; filename: string; loaded: boolean };
/** Inside a CommonJS module: `module.exports`. @group PocketOne */
declare var exports: any;
/** Inside a CommonJS module: its file path. @group PocketOne */
declare var __filename: string;
/** Inside a CommonJS module: its folder. @group PocketOne */
declare var __dirname: string;

/** @group PocketOne */
declare var process: {
  env: { [key: string]: string | undefined };
  argv: string[];
  platform: string;
};

/**
 * A small Node-style Buffer: bytes with utf8, base64, base64url, hex and latin1 conversions.
 *
 * @group PocketOne
 */
declare class Buffer extends Uint8Array {
  static from(value: string, encoding?: BufferEncoding): Buffer;
  static from(value: ArrayBuffer | ArrayLike<number> | Iterable<number>): Buffer;
  static alloc(size: number, fill?: number | string): Buffer;
  static isBuffer(value: any): value is Buffer;
  static byteLength(value: string, encoding?: BufferEncoding): number;
  static concat(list: ArrayLike<Uint8Array>): Buffer;
  toString(encoding?: BufferEncoding): string;
  toJSON(): { type: "Buffer"; data: number[] };
}
/** @group PocketOne */
type BufferEncoding = "utf8" | "utf-8" | "base64" | "base64url" | "hex" | "latin1" | "binary";
