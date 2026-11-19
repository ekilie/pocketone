package uicache

import (
	"net/http"
	"testing"
	"testing/fstest"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	"pocketone.dev/ext/extkit"
)

func TestPolicy(t *testing.T) {
	cases := map[string]string{
		"/_/":                                "",
		"/_/extensions.js":                   Revalidate,
		"/_/index.html":                      Revalidate,
		"/_/assets/index-CyzY-GNY.js":        "",
		"/_/assets/docsList-DCFjw6b9.js":     "",
		"/_/assets/index-Cx1HYrDT.css":       "",
		"/_/assets/plain.js":                 Short,
		"/_/libs/uplot/uplot.iife.js":        Short,
		"/_/fonts/remixicon/remixicon.woff2": Short,
		"/_/images/favicon.png":              Short,
		"/_/extensions/api-tokens/main.js":   Short,
		"/api/health":                        "",
		"/extensions.js":                     "",
	}
	for path, want := range cases {
		if got := Policy(path); got != want {
			t.Errorf("Policy(%q) = %q, want %q", path, got, want)
		}
	}
}

// The header it sets survives PocketBase's own dashboard middleware, which only fills an empty Cache-Control.
func TestHeadersOnServe(t *testing.T) {
	const upstream = "max-age=1209600, stale-while-revalidate=86400"
	cases := []struct {
		url  string
		want string
	}{
		{"/_/extensions.js", Revalidate},
		{"/_/extensions/demo/style.css", Short},
	}
	for _, c := range cases {
		s := tests.ApiScenario{
			Name:            c.url,
			Method:          http.MethodGet,
			URL:             c.url,
			ExpectedStatus:  200,
			ExpectedContent: []string{""},
			TestAppFactory: func(t testing.TB) *tests.TestApp {
				app, err := tests.NewTestApp()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := extkit.Setup(app, []extkit.Extension{New()}, nil, nil); err != nil {
					t.Fatal(err)
				}
				app.OnServe().BindFunc(func(se *core.ServeEvent) error {
					se.UIExtensions = append(se.UIExtensions, core.UIExtension{Name: "demo", FS: fstest.MapFS{
						"main.js":   {Data: []byte("console.log(1)")},
						"style.css": {Data: []byte("p{}")},
					}})
					return se.Next()
				})
				return app
			},
			AfterTestFunc: func(t testing.TB, _ *tests.TestApp, res *http.Response) {
				if got := res.Header.Get("Cache-Control"); got != c.want {
					t.Errorf("%s: Cache-Control %q, want %q (PocketBase's own is %q)", c.url, got, c.want, upstream)
				}
			},
		}
		s.Test(t)
	}
}
