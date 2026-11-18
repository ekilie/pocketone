// The summary module in C: a plain WASI program, built with zig cc (which brings wasi-libc).
//
//   stdin:  the post's text, as is (the hook passes a string: $wasm.run.async("summary", body))
//   stdout: {"text": "<first 12 words> …", "words": <word count>}
//
// C has no JSON in its standard library, so this one takes plain text in and prints JSON out. To take JSON in, add
// a parser such as cJSON (one .c file) beside main.c.
//
// Build (Zig 0.17; zig cc gives a WASI program a 16 MB stack unless told otherwise, and every call would start with
// it):
//
//   zig cc -target wasm32-wasi -std=c23 -Os -s -Wl,-z,stack-size=1048576 -o summary.pb.wasm main.c
#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// what `summary --describe` prints: manifest.json, built in (C23 #embed)
static const char manifest[] = {
#embed "manifest.json"
    , 0};

// fail ends the program with an error pb_hooks can read (err.code / err.message).
static void fail(const char *code, const char *message) {
  fprintf(stderr, "{\"code\":\"%s\",\"message\":\"%s\"}", code, message);
  exit(1);
}

// put_json_string prints s as a JSON string.
static void put_json_string(const char *s, size_t n) {
  putchar('"');
  for (size_t i = 0; i < n; i++) {
    unsigned char c = (unsigned char)s[i];
    if (c == '"' || c == '\\') printf("\\%c", c);
    else if (c < 0x20) printf("\\u%04x", c);
    else putchar(c);
  }
  putchar('"');
}

int main(int argc, char **argv) {
  const char *function = argc > 1 ? argv[1] : "";
  if (strcmp(function, "--describe") == 0) {
    fputs(manifest, stdout);
    return 0;
  }
  if (strcmp(function, "run") != 0) fail("no_such_function", "only run");

  // read all of stdin
  size_t len = 0, cap = 4096;
  char *text = malloc(cap);
  for (size_t n; (n = fread(text + len, 1, cap - len, stdin)) > 0;) {
    len += n;
    if (len == cap) text = realloc(text, cap *= 2);
  }

  // count the words, and remember where the 12th one ends
  size_t words = 0, short_end = 0;
  for (size_t i = 0; i < len;) {
    while (i < len && isspace((unsigned char)text[i])) i++;
    if (i == len) break;
    while (i < len && !isspace((unsigned char)text[i])) i++;
    if (++words <= 12) short_end = i;
  }
  if (words == 0) fail("empty_body", "the post has no body to summarize");

  // the first 12 words, with runs of spaces made one
  char *short_text = malloc(short_end + 8);
  size_t m = 0;
  for (size_t i = 0; i < short_end;) {
    while (i < short_end && isspace((unsigned char)text[i])) i++;
    if (m > 0 && i < short_end) short_text[m++] = ' ';
    while (i < short_end && !isspace((unsigned char)text[i])) short_text[m++] = text[i++];
  }
  if (words > 12) m += (size_t)sprintf(short_text + m, " \xe2\x80\xa6"); // " …"

  fputs("{\"text\":", stdout);
  put_json_string(short_text, m);
  printf(",\"words\":%zu}", words);
  return 0;
}
