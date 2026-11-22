// Package llm gives every PocketOne app cloud models to call, one contract per kind: llm.text/v1, llm.image/v1,
// llm.speech/v1, llm.transcribe/v1, llm.music/v1, llm.video/v1 and llm.embed/v1, and $llm in pb_hooks.
//
// The app's admins add providers in Settings → LLM (Vultr, Cloudflare Workers AI, OpenRouter, or any
// OpenAI-compatible API, each with its own key). Every call names its model as "provider/model"; this extension
// routes it to that provider's driver (providers/).
package llm

import (
	"embed"
	"io/fs"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	llmc "pocketone.dev/ext/contracts/llm"
	alignv1 "pocketone.dev/ext/contracts/llm/align/v1"
	embedv1 "pocketone.dev/ext/contracts/llm/embed/v1"
	imagev1 "pocketone.dev/ext/contracts/llm/image/v1"
	musicv1 "pocketone.dev/ext/contracts/llm/music/v1"
	speechv1 "pocketone.dev/ext/contracts/llm/speech/v1"
	textv1 "pocketone.dev/ext/contracts/llm/text/v1"
	transcribev1 "pocketone.dev/ext/contracts/llm/transcribe/v1"
	videov1 "pocketone.dev/ext/contracts/llm/video/v1"
	settingsv1 "pocketone.dev/ext/contracts/settings/v1"
	"pocketone.dev/ext/extkit"
)

// Name is the extension's key.
const Name = "llm"

// Config is the `llm` section of pocketone.yaml. Providers and keys are set in Settings → LLM, not here.
type Config struct {
	Enabled *bool `yaml:"enabled" json:"enabled,omitempty" doc:"false turns the extension off on this app ($llm and the llm.* contracts are gone)."`
}

// Section is the `llm` section.
var Section = extkit.NewSection(Name, Config{}, "Cloud models for $llm (the llm.* contracts); providers and keys are set in Settings → LLM.")

// Extension provides the llm.* contracts and Settings → LLM.
type Extension struct{}

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension.
func New() *Extension { return &Extension{} }

// Name implements extkit.Extension.
func (*Extension) Name() string { return Name }

// Contracts implements extkit.Declarer.
func (*Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{
		Implements: extkit.IDs("llm.text/v1", "llm.image/v1", "llm.speech/v1", "llm.transcribe/v1", "llm.align/v1",
			"llm.music/v1", "llm.video/v1", "llm.embed/v1"),
		// search/v1 (enrich) isn't declared: search works with llm.embed/v1, and the two declarations would order
		// each other; enrich looks search up at call time.
		WorksWith: extkit.IDs("settings/v1", "kv/v1"),
	}
}

// Register implements extkit.Extension.
func (*Extension) Register(app core.App, _ extkit.Config) error {
	r := &router{app: app}
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.UIExtensions = append(se.UIExtensions, core.UIExtension{Name: Name, FS: UI()})
		return se.Next()
	})
	settingsv1.Add(app, settingsSection{r: r})
	textv1.Contract.Provide(app, textService{r})
	imagev1.Contract.Provide(app, imageService{r})
	speechv1.Contract.Provide(app, speechService{r})
	transcribev1.Contract.Provide(app, transcribeService{r})
	alignv1.Contract.Provide(app, alignService{r})
	embedv1.Contract.Provide(app, embedService{r})
	musicv1.Contract.Provide(app, musicService{jobService{r, llmc.KindMusic}})
	videov1.Contract.Provide(app, videoService{jobService{r, llmc.KindVideo}})
	return nil
}

//go:embed ui
var uiFiles embed.FS

// UI is the dashboard's Settings → LLM screen (main.js).
func UI() fs.FS {
	sub, _ := fs.Sub(uiFiles, "ui")
	return sub
}
