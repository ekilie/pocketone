package main

import "testing"

func TestAbsLinks(t *testing.T) {
	in := "See [ctx](/docs/ctx/req#query), [post](/blog/x), [idx](/llms.txt), [ext](https://x.dev/docs/a), [rel](./a.md).\n[ref]: /docs/b\n"
	want := "See [ctx](https://p.dev/docs/ctx/req#query), [post](https://p.dev/blog/x), [idx](https://p.dev/llms.txt), [ext](https://x.dev/docs/a), [rel](./a.md).\n[ref]: https://p.dev/docs/b\n"
	if got := absLinks(in, "https://p.dev"); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestUnescapeEJSUndoesEscape(t *testing.T) {
	for _, s := range []string{"<%= x %> and <%% literal", `<script server>run()</script>`, "plain"} {
		if got := unescapeEJS(escapeEJS(s)); got != s {
			t.Errorf("round trip of %q gave %q", s, got)
		}
	}
}

func TestSplitFront(t *testing.T) {
	fm, body := splitFront("---\ntitle: \"A \\\"q\\\"\"\ndescription: plain words\n---\n\n# A\n")
	if fm["title"] != `A "q"` || fm["description"] != "plain words" || body != "# A\n" {
		t.Fatalf("got %q %q", fm, body)
	}
}

func TestParseToc(t *testing.T) {
	secs := parseToc("# Intro\n\n- [One](/docs/one)\n<!-- x -->\n# More\n- [Two](/docs/two)\n- [Out](https://x.dev)\n")
	if len(secs) != 2 || secs[0].title != "Intro" || len(secs[0].links) != 1 || secs[1].links[1].url != "https://x.dev" {
		t.Fatalf("got %+v", secs)
	}
}
