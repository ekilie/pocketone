package pages

import "testing"

// In TSX, an empty slot is falsy, so slot('title') || 'fallback' falls back; a set slot is printed unescaped.
func TestJSXEmptySlotFallsBack(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"+layout.tsx": `export default function L({ children, slot }: any) {
  return <html><head><title>{slot('title') || 'Site'}</title></head><body>{children}</body></html>
}`,
		"index.tsx": `export default () => <p>home</p>`,
		"about.tsx": `export default ({ slot }: any) => { slot('title', <b>About</b>); return <p>about</p> }`,
	})
	for path, want := range map[string]string{
		"/":      `<!doctype html><html><head><title>Site</title></head><body><p>home</p></body></html>`,
		"/about": `<!doctype html><html><head><title><b>About</b></title></head><body><p>about</p></body></html>`,
	} {
		if code, body := get(t, dir, path, false); code != 200 || body != want {
			t.Errorf("%s: %d\n%s\nwant\n%s", path, code, body, want)
		}
	}
}
