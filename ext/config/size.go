package config

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Size is a byte count written as a plain number or with a unit: B, KB, KiB, MB, MiB, GB or GiB (128MiB).
type Size int64

// ParseSize parses a Size.
func ParseSize(s string) (Size, error) {
	t := strings.TrimSpace(s)
	units := []struct {
		suffix string
		mult   float64
	}{{"KIB", 1 << 10}, {"MIB", 1 << 20}, {"GIB", 1 << 30}, {"KB", 1e3}, {"MB", 1e6}, {"GB", 1e9}, {"B", 1}}
	mult := 1.0
	for _, u := range units {
		if strings.HasSuffix(strings.ToUpper(t), u.suffix) {
			t, mult = strings.TrimSpace(t[:len(t)-len(u.suffix)]), u.mult
			break
		}
	}
	n, err := strconv.ParseFloat(t, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid size %q (such as 128MiB)", s)
	}
	return Size(n * mult), nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (s *Size) UnmarshalYAML(n *yaml.Node) error {
	v, err := ParseSize(n.Value)
	*s = v
	return err
}
