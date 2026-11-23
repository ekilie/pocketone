// Command example is a PocketBase app with search buckets: the llm extension embeds, search keeps the buckets. Add a
// provider with embedding models in Settings → LLM, run `go run . serve`, open the dashboard's Search screen, make a
// bucket and drop Markdown files on it. Or over HTTP as a superuser:
//
//	POST /api/_c/search/v1/ensure   {"name": "docs"}
//	POST /api/_c/search/v1/add      {"bucket": "docs", "name": "install.md", "text": "# Install\n..."}
//	POST /api/_c/search/v1/search   {"buckets": ["docs"], "query": "how do I install it"}
//
// In page code the same is $search.ensure(...), $search.add(...) and await $search.searchAsync(...).
package main

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"

	"pocketone.dev/ext/devusers"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/llm"
	llmsearch "pocketone.dev/ext/llm-search"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app)
	if _, err := extkit.Setup(app, []extkit.Extension{llm.New(), llmsearch.New()}, nil, nil); err != nil {
		log.Fatal(err)
	}
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
