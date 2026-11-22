package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	searchv1 "pocketone.dev/ext/contracts/search/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/js"
)

// BindJS exposes search/v1 to the app's page code as $search. Everything is local (the app's own database) and plain,
// except searching, which may ask the embedding model about the query: that one is async only and says so in its
// name (await $search.searchAsync({...})), so a model call never holds the app's one JS VM. Adding a document only
// splits it; the chunks are embedded in the background.
func (*Extension) BindJS(app core.App, rt js.Runtime) {
	n := rt.NewObject()
	_ = n.Set("call", func(op, opts string) (string, error) {
		impl, ok := searchv1.Contract.Of(app)
		if !ok {
			return "", fmt.Errorf("$search: nothing provides search/v1")
		}
		for _, o := range searchv1.W.Ops {
			if o.Name != op || o.Out != extkit.OutJSON || o.RawIn {
				continue
			}
			req := extkit.OpRequest{JSON: json.RawMessage(opts)}
			if o.Method == "GET" {
				req.JSON, req.Query = nil, queryOf(json.RawMessage(opts))
			}
			res, err := o.Invoke(context.Background(), impl, extkit.Internal, req)
			if err != nil {
				return "", err
			}
			b, err := json.Marshal(res)
			return string(b), err
		}
		return "", fmt.Errorf("$search: no operation %q", op)
	})
	if err := rt.Set(nativeGlobal, n); err != nil {
		panic(fmt.Errorf("search: bind JS: %w", err))
	}
	if _, err := rt.Eval(jsPrelude, "ext/search"); err != nil {
		panic(fmt.Errorf("search: bind JS: %w", err))
	}
}

// queryOf is a GET operation's input (a JSON object) as query values.
func queryOf(raw json.RawMessage) url.Values {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	q := url.Values{}
	for k, v := range m {
		if v != nil {
			q.Set(k, fmt.Sprint(v))
		}
	}
	return q
}

const nativeGlobal = "__pocketone_search"

// jsPrelude builds $search over the Go function, which it takes off the global scope.
const jsPrelude = `(() => {
  const n = globalThis.__pocketone_search;
  delete globalThis.__pocketone_search;
  const S = (o) => JSON.stringify(o ?? {});
  const call = (op, o) => JSON.parse(n.call(op, S(o)));
  const callAsync = async (op, o) => JSON.parse(await n.call.async(op, S(o)));
  const ref = (x) => (typeof x === "string" ? { name: x } : x);
  const names = (x) => (x == null ? [] : [].concat(x));
  globalThis.$search = Object.freeze({
    buckets: () => call("buckets").buckets,
    bucket: (name) => call("bucket", ref(name)),
    ensure: (o) => call("ensure", ref(o)),
    update: (o) => call("update", o),
    deleteBucket: (name) => call("delete", ref(name)).count,
    add: (o) => call("add", o),
    documents: (x) => call("documents", typeof x === "string" ? { bucket: x } : x).documents,
    remove: (bucket, list) => call("remove", { bucket, names: names(list) }).count,
    reindex: (bucket, list) => call("reindex", { bucket, names: names(list) }).count,
    searchAsync: (o) => callAsync("search", o),
  });
})()`

// TypeScript is $search's declarations for the app's types.d.ts.
func (*Extension) TypeScript() string { return typeScript }

const typeScript = `type SearchBucket = { name: string; title: string; description: string; owner: string; model: string; dimensions: number;
  maxDocuments: number; maxBytes: number; documents: number; chunks: number; bytes: number; pending: number; errors: number;
  created: string; updated: string }

type SearchDocument = { bucket: string; name: string; kind: "markdown" | "code" | "text"; size: number; hash: string;
  status: "queued" | "embedding" | "ready" | "error"; error?: string; chunks: number; tokens: number; costMicroUSD: number;
  unchanged?: boolean; created: string; updated: string }

type SearchResult = { text: string; document: string; heading?: string; startLine: number; endLine: number;
  score: number; semantic?: number; keyword?: number }

/** Search buckets (ext/search, the search/v1 contract). Searching is async; everything else is local. */
declare var $search: {
  /** Every bucket, by name. */
  buckets(): SearchBucket[]
  bucket(name: string): SearchBucket
  /** Makes the bucket if it's missing (call it at startup for the bucket your app needs) and returns it. */
  ensure(o: string | { name: string; title?: string; description?: string; owner?: string; model?: string; dimensions?: number }): SearchBucket
  update(o: { name: string; title?: string; description?: string; model?: string; dimensions?: number; maxDocuments?: number; maxBytes?: number }): SearchBucket
  /** Deletes a bucket and everything in it. */
  deleteBucket(name: string): number
  /** Adds (or replaces) a document by name; it's embedded in the background. */
  add(o: { bucket: string; name: string; text: string }): SearchDocument
  documents(bucket: string | { bucket: string; prefix?: string }): SearchDocument[]
  remove(bucket: string, names: string | string[]): number
  /** Splits and embeds documents again (all of them when names is left out). */
  reindex(bucket: string, names?: string | string[]): number
  /** The best matches for a query, best first. mode: "hybrid" (default), "semantic" or "keyword". */
  searchAsync(o: { bucket: string; query: string; limit?: number; mode?: "hybrid" | "semantic" | "keyword"; prefix?: string }):
    Promise<{ results: SearchResult[]; mode: string; model?: string; usage: { costMicroUSD: number; inputTokens?: number } }>
}
`
