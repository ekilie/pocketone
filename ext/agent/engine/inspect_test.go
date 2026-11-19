package engine

import (
	"fmt"
	"os"
	"sort"
	"testing"
)

func TestInspect(t *testing.T) {
	p := os.Getenv("GGUF")
	if p == "" {
		t.Skip("GGUF not set")
	}
	g, err := openGGUF(p)
	if err != nil {
		t.Fatal(err)
	}
	defer g.close()
	keys := []string{}
	for k := range g.KV {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := g.KV[k]
		switch a := v.(type) {
		case []string:
			fmt.Printf("%s = [%d strings] %q\n", k, len(a), a[:min(5, len(a))])
		case []int32:
			fmt.Printf("%s = [%d int32] %v\n", k, len(a), a[:min(5, len(a))])
		case []any:
			fmt.Printf("%s = [%d any]\n", k, len(a))
		default:
			fmt.Printf("%s = %v\n", k, v)
		}
	}
	counts := map[string]int{}
	for _, n := range g.Order {
		ti := g.Tensors[n]
		if !(len(n) > 4 && n[:4] == "blk.") || n[:6] == "blk.0." {
			fmt.Println(n, ti.Dims, typeName(ti.Type))
		}
		counts[typeName(ti.Type)]++
	}
	fmt.Println(counts)
}
