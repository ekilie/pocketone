// Command example is a PocketBase app with search buckets: the llm extension embeds, search keeps the buckets. Add a
// provider in Settings → LLM, run `go run . serve`, open the dashboard's Search screen, make a bucket with an
// embedding model ("vultr/qwen3-embedding-4b") and drop Markdown files on it. Or over HTTP as a superuser:
//
//	POST /api/_c/search/v1/ensure   {"name": "docs", "model": "vultr/qwen3-embedding-4b", "dimensions": 1024}
//	POST /api/_c/search/v1/add      {"bucket": "docs", "name": "install.md", "text": "# Install\n..."}
//	POST /api/_c/search/v1/search   {"bucket": "docs", "query": "how do I install it"}
//
// In page code the same is $search.ensure(...), $search.add(...) and await $search.searchAsync(...).
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"

	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/llm"
	"pocketone.dev/ext/search"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app)
	if _, err := extkit.Setup(app, []extkit.Extension{llm.New(), search.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
