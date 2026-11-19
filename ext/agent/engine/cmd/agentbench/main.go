// Command agentbench compares the pure-Go engine with Ollama on the same GGUF files.
//
//	agentbench compare [-models qwen3:0.6b,qwen3:1.7b]       token counts + greedy agreement
//	agentbench bench   [-models ...] [-runs 3] [-gen 128]     full benchmark table
//	agentbench requant -in qwen3:0.6b -out FILE.gguf                (write a Q8_0 copy for testing)
//	agentbench run     -gguf FILE -threads N -prompt short|long -gen 128   (one engine run, JSON)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"pocketone.dev/ext/agent/engine"
	"pocketone.dev/ext/agent/engine/internal/benchhooks"
)

var ollamaURL = "http://127.0.0.1:11434"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: agentbench compare|bench|run [flags]")
		os.Exit(2)
	}
	if u := os.Getenv("OLLAMA_URL"); u != "" {
		ollamaURL = u
	}
	var err error
	switch os.Args[1] {
	case "run":
		err = cmdRun(os.Args[2:])
	case "compare":
		err = cmdCompare(os.Args[2:])
	case "bench":
		err = cmdBench(os.Args[2:])
	case "requant":
		fs := flag.NewFlagSet("requant", flag.ExitOnError)
		in := fs.String("in", "", "input GGUF (or Ollama tag)")
		out := fs.String("out", "", "output GGUF")
		fs.Parse(os.Args[2:])
		var p string
		if p, err = resolveModel(*in); err == nil {
			err = benchhooks.RequantizeQ8_0(p, *out)
		}
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func logf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, time.Now().Format("15:04:05")+" "+format+"\n", a...)
}

func promptText(name string, run int) string {
	p := shortPrompt
	if name == "long" {
		p = longPrompt
	}
	if run > 0 {
		// a unique first line defeats Ollama's prompt cache between runs
		p = fmt.Sprintf("Draft %d.\n", run) + p
	}
	return p
}

// resolveModel maps an Ollama tag (or a path to a .gguf file) to a GGUF path.
func resolveModel(tag string) (string, error) {
	if strings.HasSuffix(tag, ".gguf") || strings.HasPrefix(tag, "/") {
		return tag, nil
	}
	name, ver, ok := strings.Cut(tag, ":")
	if !ok {
		ver = "latest"
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ollama", "models")
	if d := os.Getenv("OLLAMA_MODELS"); d != "" {
		dir = d
	}
	b, err := os.ReadFile(filepath.Join(dir, "manifests", "registry.ollama.ai", "library", name, ver))
	if err != nil {
		return "", err
	}
	var man struct {
		Layers []struct{ MediaType, Digest string }
	}
	if err := json.Unmarshal(b, &man); err != nil {
		return "", err
	}
	for _, l := range man.Layers {
		if l.MediaType == "application/vnd.ollama.image.model" {
			return filepath.Join(dir, "blobs", strings.Replace(l.Digest, ":", "-", 1)), nil
		}
	}
	return "", fmt.Errorf("no model layer in manifest for %s", tag)
}

// ---- our engine, one run per process so peak RSS is per configuration ----

type runResult struct {
	Load         float64 `json:"load_s"`
	PromptTokens []int   `json:"prompt_tokens"`
	Out          []int   `json:"out_tokens"`
	Prefill      []float64
	Decode       []float64
	TTFT         []float64
	Text         string
	Tokens       []int
	MaxRSS       int64
	Kernels      string
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	gguf := fs.String("gguf", "", "model file")
	threads := fs.Int("threads", 8, "threads")
	prompt := fs.String("prompt", "short", "short|long")
	gen := fs.Int("gen", 128, "tokens to generate")
	runs := fs.Int("runs", 3, "measured runs (after one warm-up)")
	fs.Parse(args)
	t0 := time.Now()
	m, err := engine.Load(*gguf, engine.LoadOptions{Threads: *threads})
	if err != nil {
		return err
	}
	defer m.Close()
	r := runResult{Load: time.Since(t0).Seconds(), Kernels: m.Info().Kernels}
	for i := 0; i <= *runs; i++ {
		p := promptText(*prompt, i)
		res, err := m.Generate(context.Background(), p, engine.GenerateOptions{MaxTokens: *gen, IgnoreEOS: true}, nil)
		if err != nil {
			return err
		}
		ttft := res.Prefill
		label := fmt.Sprintf("run %d/%d", i, *runs)
		if i == 0 {
			label = "warm-up"
		}
		logf("%s: %d prompt tokens, prefill %.1f tok/s, %d tokens decoded at %.1f tok/s", label, res.PromptTokens,
			float64(res.PromptTokens)/res.Prefill.Seconds(), res.OutputTokens, float64(res.OutputTokens-1)/res.Decode.Seconds())
		if i == 0 {
			continue // warm-up (pages the weights in)
		}
		r.PromptTokens = append(r.PromptTokens, res.PromptTokens)
		r.Out = append(r.Out, res.OutputTokens)
		r.Prefill = append(r.Prefill, float64(res.PromptTokens)/res.Prefill.Seconds())
		r.Decode = append(r.Decode, float64(res.OutputTokens-1)/res.Decode.Seconds())
		r.TTFT = append(r.TTFT, ttft.Seconds())
		r.Text, r.Tokens = res.Text, res.Tokens
	}
	r.MaxRSS = maxRSS()
	fmt.Fprintf(os.Stderr, "kernels %s, threads %d, load %.2fs, prompt %v tokens, prefill %.1f tok/s, decode %.1f tok/s, TTFT %.0f ms, peak RSS %d MB (medians of %d runs)\n",
		r.Kernels, *threads, r.Load, r.PromptTokens, median(r.Prefill), median(r.Decode), median(r.TTFT)*1000, r.MaxRSS>>20, len(r.Prefill))
	return json.NewEncoder(os.Stdout).Encode(r)
}

// ---- Ollama ----

type ollamaResp struct {
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	DoneReason         string `json:"done_reason"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
	TotalDuration      int64  `json:"total_duration"`
	Logprobs           []struct {
		Token   string  `json:"token"`
		Logprob float64 `json:"logprob"`
		Bytes   []byte  `json:"bytes"`
	} `json:"logprobs"`
	Error string `json:"error"`
}

func ollamaGenerate(model, prompt string, opts map[string]any, timeout time.Duration) (*ollamaResp, error) {
	rq := map[string]any{"model": model, "prompt": prompt, "raw": true, "stream": false, "options": opts, "keep_alive": "10m"}
	if lp, ok := opts["_logprobs"]; ok {
		delete(opts, "_logprobs")
		rq["logprobs"] = lp
	}
	body, _ := json.Marshal(rq)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", ollamaURL+"/api/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var r ollamaResp
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("ollama: %s", b)
	}
	if r.Error != "" {
		return nil, fmt.Errorf("ollama: %s", r.Error)
	}
	return &r, nil
}

func ollamaUnloadAll(models []string) {
	for _, m := range models {
		body, _ := json.Marshal(map[string]any{"model": m, "keep_alive": 0})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "POST", ollamaURL+"/api/generate", bytes.NewReader(body))
		if resp, err := http.DefaultClient.Do(req); err == nil {
			resp.Body.Close()
		}
		cancel()
	}
	time.Sleep(time.Second)
}

// runnerRSS returns the summed RSS (bytes) of `ollama runner` processes.
func runnerRSS() int64 {
	args := []string{"-axo", "rss=,command="} // BSD/macOS
	if runtime.GOOS == "linux" {
		args = []string{"-eo", "rss=,args="}
	}
	out, err := exec.Command("ps", args...).Output()
	if err != nil {
		return 0
	}
	var total int64
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "ollama") || !strings.Contains(line, "runner") {
			continue
		}
		f := strings.Fields(line)
		if kb, err := strconv.ParseInt(f[0], 10, 64); err == nil {
			total += kb * 1024
		}
	}
	return total
}

// ---- compare ----

// cmpThreads is the thread count for correctness comparisons (speed doesn't matter there).
var cmpThreads = min(8, runtime.NumCPU())

func greedyOpts(gen int) map[string]any {
	return map[string]any{"temperature": 0, "top_k": 1, "seed": 1, "num_predict": gen, "num_ctx": 2048}
}

func cmdCompare(args []string) error {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	models := fs.String("models", "qwen3:0.6b,qwen3:1.7b", "Ollama tags (or .gguf paths with -ollama-names)")
	gen := fs.Int("gen", 64, "tokens to compare")
	cpu := fs.Bool("cpu", true, "run Ollama on CPU (num_gpu 0) for the comparison")
	fs.Parse(args)
	fmt.Println("| model | prompt | our tokens | ollama prompt_eval_count | equal | free-running greedy match | teacher-forced top-1 agreement | logprob abs diff mean / max |")
	fmt.Println("|---|---|---|---|---|---|---|---|")
	for _, tag := range strings.Split(*models, ",") {
		path, err := resolveModel(tag)
		if err != nil {
			return err
		}
		m, err := engine.Load(path, engine.LoadOptions{Threads: cmpThreads})
		if err != nil {
			return err
		}
		for _, pn := range []string{"short", "long"} {
			p := promptText(pn, 0)
			ours := m.Tokenize(p)
			opts := greedyOpts(*gen)
			opts["_logprobs"] = true
			if *cpu {
				opts["num_gpu"] = 0
				opts["num_thread"] = cmpThreads
			}
			logf("ollama %s %s ...", tag, pn)
			or, err := ollamaGenerate(tag, p, opts, 5*time.Minute)
			if err != nil {
				return err
			}
			res, err := m.Generate(context.Background(), p, engine.GenerateOptions{MaxTokens: *gen}, nil)
			if err != nil {
				return err
			}
			// tokens of ours that lie entirely within the common text prefix
			common := 0
			for common < len(res.Text) && common < len(or.Response) && res.Text[common] == or.Response[common] {
				common++
			}
			match, off := 0, 0
			for _, id := range res.Tokens {
				off += len(m.Detokenize([]int{id}))
				if off > common {
					break
				}
				match++
			}
			if common == len(res.Text) && common == len(or.Response) {
				match = len(res.Tokens)
			}
			// teacher-forced: feed Ollama's tokens, check our argmax and logprob at each step
			var cont []int
			var theirLP []float64
			for _, lp := range or.Logprobs {
				ids := m.Tokenize(string(lp.Bytes))
				if len(ids) != 1 {
					break
				}
				cont = append(cont, ids[0])
				theirLP = append(theirLP, lp.Logprob)
			}
			am, ourLP := benchhooks.TeacherForce(m, ours, cont)
			agree := 0
			var maxd, sumd float64
			for i := range cont {
				if am[i] == cont[i] {
					agree++
				}
				d := ourLP[i] - theirLP[i]
				if d < 0 {
					d = -d
				}
				sumd += d
				maxd = max(maxd, d)
			}
			fmt.Printf("| %s | %s | %d | %d | %v | %d/%d | %d/%d | %.3f / %.3f |\n", tag, pn, len(ours), or.PromptEvalCount,
				len(ours) == or.PromptEvalCount, match, len(res.Tokens), agree, len(cont), sumd/float64(max(1, len(cont))), maxd)
			if match < len(res.Tokens) && os.Getenv("VERBOSE") != "" {
				fmt.Printf("    ours:   %q\n    ollama: %q\n", res.Text, or.Response)
			}
		}
		m.Close()
	}
	return nil
}

// ---- bench ----

type row struct {
	Model, Prompt, Engine       string
	PromptTok, OutTok           int
	Prefill, Decode, TTFT, Load float64
	RSS                         int64
	Note                        string
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[len(s)/2]
}

func medianInt(v []int) int {
	f := make([]float64, len(v))
	for i, x := range v {
		f[i] = float64(x)
	}
	return int(median(f))
}

func cmdBench(args []string) error {
	fs := flag.NewFlagSet("bench", flag.ExitOnError)
	models := fs.String("models", "qwen3:0.6b,qwen3:1.7b", "Ollama tags, or .gguf paths (ours only)")
	runs := fs.Int("runs", 3, "measured runs")
	gen := fs.Int("gen", 128, "tokens to generate")
	threadsList := fs.String("threads", "8,4", "thread counts")
	engines := fs.String("engines", "ours,ollama-cpu,ollama-gpu", "which engines")
	fs.Parse(args)
	exe, _ := os.Executable()
	var rows []row
	tags := strings.Split(*models, ",")
	want := map[string]bool{}
	for _, e := range strings.Split(*engines, ",") {
		want[e] = true
	}
	var ths []int
	for _, t := range strings.Split(*threadsList, ",") {
		n, _ := strconv.Atoi(t)
		ths = append(ths, n)
	}
	for _, tag := range tags {
		path, err := resolveModel(tag)
		if err != nil {
			return err
		}
		isOllama := !strings.HasSuffix(tag, ".gguf")
		for _, pn := range []string{"short", "long"} {
			if want["ours"] {
				for _, th := range ths {
					logf("ours %s %s threads=%d", tag, pn, th)
					cmd := exec.Command(exe, "run", "-gguf", path, "-threads", strconv.Itoa(th), "-prompt", pn,
						"-gen", strconv.Itoa(*gen), "-runs", strconv.Itoa(*runs))
					cmd.Stderr = os.Stderr
					out, err := cmd.Output()
					if err != nil {
						return fmt.Errorf("run: %v", err)
					}
					var r runResult
					if err := json.Unmarshal(out, &r); err != nil {
						return err
					}
					rows = append(rows, row{tag, pn, fmt.Sprintf("ours t=%d", th), medianInt(r.PromptTokens), medianInt(r.Out),
						median(r.Prefill), median(r.Decode), median(r.TTFT), r.Load, r.MaxRSS, r.Kernels})
					logf("  -> prefill %.1f tok/s, decode %.1f tok/s", median(r.Prefill), median(r.Decode))
				}
			}
			if !isOllama {
				continue
			}
			type ocfg struct {
				name string
				opts map[string]any
			}
			var cfgs []ocfg
			if want["ollama-cpu"] {
				for _, th := range ths {
					o := greedyOpts(*gen)
					o["num_gpu"] = 0
					o["num_thread"] = th
					cfgs = append(cfgs, ocfg{fmt.Sprintf("ollama cpu t=%d", th), o})
				}
			}
			if want["ollama-gpu"] {
				cfgs = append(cfgs, ocfg{"ollama metal", greedyOpts(*gen)})
			}
			for _, c := range cfgs {
				logf("%s %s %s", c.name, tag, pn)
				ollamaUnloadAll(tags)
				var pre, dec, ttft []float64
				var pt, ot []int
				var load float64
				var peak int64
				done := make(chan struct{})
				go func() {
					t := time.NewTicker(100 * time.Millisecond)
					defer t.Stop()
					for {
						select {
						case <-done:
							return
						case <-t.C:
							if r := runnerRSS(); r > peak {
								peak = r
							}
						}
					}
				}()
				var note string
				for i := 0; i <= *runs; i++ {
					r, err := ollamaGenerate(tag, promptText(pn, i), c.opts, 10*time.Minute)
					if err != nil {
						close(done)
						return err
					}
					if i == 0 {
						load = float64(r.LoadDuration) / 1e9
						continue
					}
					pt = append(pt, r.PromptEvalCount)
					ot = append(ot, r.EvalCount)
					pre = append(pre, float64(r.PromptEvalCount)/(float64(r.PromptEvalDuration)/1e9))
					dec = append(dec, float64(r.EvalCount)/(float64(r.EvalDuration)/1e9))
					ttft = append(ttft, float64(r.PromptEvalDuration)/1e9)
					if r.EvalCount < *gen {
						note = "stopped early: " + r.DoneReason
					}
				}
				close(done)
				rows = append(rows, row{tag, pn, c.name, medianInt(pt), medianInt(ot), median(pre), median(dec), median(ttft), load, peak, note})
				logf("  -> prefill %.1f tok/s, decode %.1f tok/s", median(pre), median(dec))
			}
		}
	}
	ollamaUnloadAll(tags)
	fmt.Println("| model | prompt | engine | prompt tok | out tok | prefill tok/s | decode tok/s | TTFT | load | peak RSS | note |")
	fmt.Println("|---|---|---|---|---|---|---|---|---|---|---|")
	for _, r := range rows {
		fmt.Printf("| %s | %s | %s | %d | %d | %.1f | %.1f | %.0f ms | %.2f s | %.0f MB | %s |\n", r.Model, r.Prompt, r.Engine,
			r.PromptTok, r.OutTok, r.Prefill, r.Decode, r.TTFT*1000, r.Load, float64(r.RSS)/(1<<20), r.Note)
	}
	return nil
}
