package restorev1

import (
	"embed"
	"io/fs"
)

//go:embed ui
var uiFiles embed.FS

// The restore screen on Settings → Backups works for every provider: it reads Points and calls RestoreTo.
var _ = func() bool {
	sub, _ := fs.Sub(uiFiles, "ui")
	Contract.UI("restore", sub)
	return true
}()
