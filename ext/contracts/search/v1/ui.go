package searchv1

import (
	"embed"
	"io/fs"
)

//go:embed ui
var uiFiles embed.FS

// The Search screen (a "Search" link in the dashboard's header) works for every provider: it calls only search/v1.
var _ = func() bool {
	sub, _ := fs.Sub(uiFiles, "ui")
	Contract.UI("search", sub)
	return true
}()
