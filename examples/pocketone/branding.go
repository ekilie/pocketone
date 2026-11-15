package main

import (
	"embed"
	"io/fs"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

//go:embed branding
var brandingFiles embed.FS

// brandingUI is the superuser dashboard extension that shows PocketOne's logo, favicon and version instead of
// PocketBase's (served at /_/extensions/pocketone/). It uses the dashboard's own extension seam, so core is untouched.
// The version shown is read from the X-Powered-By header this binary sends (see PoweredBy).
func brandingUI() core.UIExtension {
	sub, _ := fs.Sub(brandingFiles, "branding")
	return core.UIExtension{Name: "pocketone", FS: sub}
}
