package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// knownModel is a model PocketOne can fetch by name: a GGUF file from its publisher, checked against its SHA-256.
type knownModel struct {
	Name   string
	URL    string
	SHA256 string
	Size   int64
	About  string
}

// knownModels are the models `agent.model` can name. Each is a model the built-in runner supports, from its
// publisher's own repository.
var knownModels = []knownModel{
	{Name: "qwen3-0.6b", About: "Qwen3 0.6B, 8-bit: the smallest; tagging and short extraction",
		URL:    "https://huggingface.co/Qwen/Qwen3-0.6B-GGUF/resolve/main/Qwen3-0.6B-Q8_0.gguf",
		SHA256: "9465e63a22add5354d9bb4b99e90117043c7124007664907259bd16d043bb031", Size: 639446688},
	{Name: "qwen3-1.7b", About: "Qwen3 1.7B, 8-bit: a good default for a small server",
		URL:    "https://huggingface.co/Qwen/Qwen3-1.7B-GGUF/resolve/main/Qwen3-1.7B-Q8_0.gguf",
		SHA256: "061b54daade076b5d3362dac252678d17da8c68f07560be70818cace6590cb1a", Size: 1834426016},
	{Name: "qwen3-4b", About: "Qwen3 4B, 4-bit: better answers, needs about 3 GB of memory",
		URL:    "https://huggingface.co/Qwen/Qwen3-4B-GGUF/resolve/main/Qwen3-4B-Q4_K_M.gguf",
		SHA256: "7485fe6f11af29433bc51cab58009521f205840f5b4ae3a32fa7f92e8534fdf5", Size: 2497280256},
}

func findKnown(name string) (knownModel, bool) {
	for _, m := range knownModels {
		if m.Name == name {
			return m, true
		}
	}
	return knownModel{}, false
}

func knownNames() string {
	names := make([]string, len(knownModels))
	for i, m := range knownModels {
		names[i] = m.Name
	}
	return strings.Join(names, ", ")
}

// modelSource is where a model's file comes from: a local path (used as it is), or a download into the cache.
type modelSource struct {
	Name   string // what the config said
	Path   string // the local file
	URL    string // "" for a local file
	SHA256 string // "" when unverified (an hf: model)
	Size   int64
}

// resolveModel turns `agent.model` into a source: a known name, "hf:<org>/<repo>/<file>.gguf", or a path to a .gguf
// file.
func resolveModel(spec, cacheDir string) (modelSource, error) {
	spec = strings.TrimSpace(spec)
	if m, ok := findKnown(spec); ok {
		return modelSource{Name: spec, Path: filepath.Join(cacheDir, m.Name+".gguf"), URL: m.URL, SHA256: m.SHA256, Size: m.Size}, nil
	}
	if rest, ok := strings.CutPrefix(spec, "hf:"); ok {
		parts := strings.SplitN(rest, "/", 3)
		if len(parts) != 3 || !strings.HasSuffix(parts[2], ".gguf") {
			return modelSource{}, fmt.Errorf("agent.model %q: want hf:<org>/<repo>/<file>.gguf", spec)
		}
		file := strings.NewReplacer("/", "_", "..", "_").Replace(rest)
		return modelSource{Name: spec, Path: filepath.Join(cacheDir, "hf", file),
			URL: "https://huggingface.co/" + parts[0] + "/" + parts[1] + "/resolve/main/" + parts[2]}, nil
	}
	if strings.HasSuffix(spec, ".gguf") || strings.ContainsRune(spec, os.PathSeparator) {
		return modelSource{Name: strings.TrimSuffix(filepath.Base(spec), ".gguf"), Path: expandHome(spec)}, nil
	}
	return modelSource{}, fmt.Errorf("agent.model %q: not a model PocketOne knows (%s), an hf:<org>/<repo>/<file>.gguf, or a path to a .gguf file", spec, knownNames())
}

func expandHome(p string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, rest)
		}
	}
	return p
}

// fetch makes sure src's file is on disk, downloading it (to a .part file, then renamed) when it isn't, and checking
// its SHA-256 when known. progress gets the share done, 0..1.
func fetch(src modelSource, client *http.Client, progress *atomic.Value) error {
	if _, err := os.Stat(src.Path); err == nil {
		return nil
	} else if src.URL == "" {
		return fmt.Errorf("agent.model: %s: %w", src.Path, err)
	}
	if err := os.MkdirAll(filepath.Dir(src.Path), 0o755); err != nil {
		return err
	}
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Get(src.URL)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", src.Name, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: %s said %s", src.Name, src.URL, res.Status)
	}
	total := src.Size
	if total == 0 {
		total = res.ContentLength
	}
	part := src.Path + ".part"
	f, err := os.Create(part)
	if err != nil {
		return err
	}
	h := sha256.New()
	pw := &progressWriter{total: total, progress: progress}
	_, err = io.Copy(io.MultiWriter(f, h, pw), res.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(part)
		return fmt.Errorf("downloading %s: %w", src.Name, err)
	}
	if src.SHA256 != "" {
		if got := hex.EncodeToString(h.Sum(nil)); got != src.SHA256 {
			_ = os.Remove(part)
			return fmt.Errorf("downloading %s: checksum %s, want %s", src.Name, got, src.SHA256)
		}
	}
	return os.Rename(part, src.Path)
}

type progressWriter struct {
	total, done int64
	progress    *atomic.Value
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.total > 0 && p.progress != nil {
		p.progress.Store(float64(p.done) / float64(p.total))
	}
	return len(b), nil
}

var errClosed = errors.New("agent: the model was unloaded")
