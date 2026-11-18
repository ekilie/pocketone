// Command stats is an example ext/wasm module in TinyGo that keeps no state of its own: the caller holds it. Every
// call gets a fresh module, so the running statistics of a series (count, mean, spread, min, max) live in the app,
// in a record's JSON field, and each call takes the state in and gives the new state back (Welford's algorithm, so
// the state stays five numbers however many values it has seen).
//
//	add:    stdin {"state": <state or null>, "values": [1.5, 2]}  ->  stdout <new state>
//	report: stdin <state>                                         ->  stdout {"count", "mean", "stddev", "min", "max"}
//
// Build it with TinyGo 0.41 (with a Go it supports, 1.26):
//
//	tinygo build -target=wasip1 -opt=z -no-debug -scheduler=none -o stats.pb.wasm .
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

//go:embed manifest.json
var manifest string // what `stats --describe` prints

// State is everything the module needs to go on where the last call stopped. The app stores it as JSON.
type State struct {
	Count int     `json:"count"`
	Mean  float64 `json:"mean"`
	M2    float64 `json:"m2"` // the sum of squared distances from the mean
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

func main() {
	switch os.Args[1] {
	case "--describe":
		fmt.Print(manifest)
	case "add":
		var in struct {
			State  *State    `json:"state"`
			Values []float64 `json:"values"`
		}
		decode(&in)
		s := State{}
		if in.State != nil {
			s = *in.State
		}
		for _, v := range in.Values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				fail("bad_value", "values must be finite numbers")
			}
			if s.Count == 0 || v < s.Min {
				s.Min = v
			}
			if s.Count == 0 || v > s.Max {
				s.Max = v
			}
			s.Count++
			d := v - s.Mean
			s.Mean += d / float64(s.Count)
			s.M2 += d * (v - s.Mean)
		}
		encode(s)
	case "report":
		var s State
		decode(&s)
		if s.Count == 0 {
			fail("no_values", "the series has no values yet")
		}
		encode(map[string]any{
			"count": s.Count, "mean": s.Mean, "stddev": math.Sqrt(s.M2 / float64(s.Count)), "min": s.Min, "max": s.Max,
		})
	default:
		fail("no_such_function", os.Args[1])
	}
}

func decode(v any) {
	if err := json.NewDecoder(os.Stdin).Decode(v); err != nil {
		fail("bad_input", err.Error())
	}
}

func encode(v any) { json.NewEncoder(os.Stdout).Encode(v) }

// fail ends the program with an error pb_hooks can read (err.code / err.message).
func fail(code, message string) {
	b, _ := json.Marshal(map[string]string{"code": code, "message": message})
	os.Stderr.Write(b)
	os.Exit(1)
}
