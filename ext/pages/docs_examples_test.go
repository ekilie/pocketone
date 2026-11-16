package pages

import "testing"

// The examples on the JSX and errors docs pages work as written.
func TestDocsJSXAndNotFoundExamples(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"+layout.tsx": `export default function Layout({ children, slot }: any) {
  return (
    <html>
      <head>
        <title>{slot('title')}</title>
      </head>
      <body>{children}</body>
    </html>
  )
}`,
		"products/_private/Price.tsx": "export default ({ cents }: { cents: number }) => <span class=\"price\">${(cents / 100).toFixed(2)}</span>",
		"products/index.tsx": `import Price from './_private/Price'
export default function P({ slot }: any) {
  slot('title', 'Widget')
  return <article class="product"><h1>Widget</h1><Price cents={1999} /></article>
}`,
		"ejs.ejs": `<p>ejs in tsx</p>`,
		"+404.ejs": `<% slot('title', () => { %>Not found<% }) %><h1>Not found</h1><p>Nothing lives at <%= ctx.req.url.pathname %>.</p>`,
	})
	want := `<!doctype html><html><head><title>Widget</title></head><body><article class="product"><h1>Widget</h1><span class="price">$19.99</span></article></body></html>`
	if code, body := get(t, dir, "/products", true); code != 200 || body != want {
		t.Fatalf("/products: %d\n%s\nwant\n%s", code, body, want)
	}
	if code, body := get(t, dir, "/ejs", true); code != 200 || body != `<!doctype html><html><head><title></title></head><body><p>ejs in tsx</p></body></html>` {
		t.Fatalf("/ejs: %d %s", code, body)
	}
	if code, body := get(t, dir, "/nope", true); code != 404 || body != `<!doctype html><html><head><title>Not found</title></head><body><h1>Not found</h1><p>Nothing lives at /nope.</p></body></html>` {
		t.Fatalf("/nope: %d %s", code, body)
	}
}
