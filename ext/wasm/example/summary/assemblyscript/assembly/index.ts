// The summary module in AssemblyScript: a plain WASI program, made one by @assemblyscript/wasi-shim (process.argv,
// process.stdin, process.stdout, process.exit).
//
//   stdin:  the post's text, as is (the hook passes a string: $wasm.run.async("summary", body))
//   stdout: {"text": "<first 12 words> …", "words": <word count>}
//
// AssemblyScript's standard library has no JSON, so, like the C example, this one takes plain text in and prints JSON
// out. To take JSON in, add a JSON package for AssemblyScript (json-as, assemblyscript-json).
//
// Build (AssemblyScript 0.28, wasi-shim 0.1; see asconfig.json):
//
//   npm install
//   npx asc assembly/index.ts --target release   # writes summary.pb.wasm

// what `summary --describe` prints. AssemblyScript can't embed a file, so it's here; the extension's tests check it's
// the same as manifest.json.
const manifest = `{
  "description": "Summarizes a text: its first words, and a word count",
  "functions": ["run"],
  "language": "AssemblyScript",
  "repository": "https://github.com/pocketonehq/pocketone/tree/main/ext/wasm/example/summary/assemblyscript",
  "examples": [
    {
      "function": "run",
      "input": "one two three",
      "output": {"text": "one two three", "words": 3}
    }
  ]
}
`

// fail ends the program with an error pb_hooks can read (err.code / err.message).
function fail(code: string, message: string): void {
  process.stderr.write(`{"code":"${code}","message":"${message}"}`)
  process.exit(1)
}

// readStdin reads all of stdin as UTF-8 text.
function readStdin(): string {
  let data = new Uint8Array(4096)
  let len = 0
  while (true) {
    if (len == data.length) {
      const bigger = new Uint8Array(data.length * 2)
      bigger.set(data)
      data = bigger
    }
    const n = process.stdin.read(data.buffer, len)
    if (n <= 0) break
    len += n
  }
  return String.UTF8.decode(data.buffer.slice(0, len))
}

function isSpace(c: i32): bool {
  return c == 0x20 || c == 0x09 || c == 0x0a || c == 0x0d
}

// jsonString is s as a JSON string.
function jsonString(s: string): string {
  let out = '"'
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i)
    if (c == 0x22 || c == 0x5c) out += "\\" + String.fromCharCode(c)
    else if (c < 0x20) out += "\\u" + c.toString(16).padStart(4, "0")
    else out += String.fromCharCode(c)
  }
  return out + '"'
}

const fn = process.argv.length > 1 ? process.argv[1] : ""
if (fn == "--describe") {
  process.stdout.write(manifest)
} else if (fn != "run") {
  fail("no_such_function", "only run")
} else {
  const text = readStdin()
  const words: string[] = []
  let start = -1
  for (let i = 0; i <= text.length; i++) {
    const space = i == text.length || isSpace(text.charCodeAt(i))
    if (space && start >= 0) {
      words.push(text.substring(start, i))
      start = -1
    } else if (!space && start < 0) {
      start = i
    }
  }
  if (words.length == 0) {
    fail("empty_body", "the post has no body to summarize")
  } else {
    let short = words.slice(0, 12).join(" ")
    if (words.length > 12) short += " …"
    process.stdout.write(`{"text":${jsonString(short)},"words":${words.length}}`)
  }
}
