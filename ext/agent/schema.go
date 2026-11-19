package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pockethost/pocketone/ext/agent/engine"
)

// schemaNode is the part of a JSON schema the built-in runner enforces while it writes: type (one or a list), enum,
// const, properties, required, additionalProperties, items, minItems, maxItems, anyOf and oneOf. Other keywords are
// ignored (allowed, not checked).
type schemaNode struct {
	types    map[string]bool // empty: any
	literals []string        // enum/const values as JSON text; nil: no enum
	props    map[string]*schemaNode
	order    []string // property names, as written
	required map[string]bool
	closed   bool // no properties beyond props
	items    *schemaNode
	minItems int
	maxItems int // -1: no limit
	anyOf    []*schemaNode
}

// compileSchema reads a JSON schema for constrained output.
func compileSchema(raw json.RawMessage) (*schemaNode, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return compileNode(v)
}

func compileNode(v any) (*schemaNode, error) {
	n := &schemaNode{maxItems: -1}
	m, ok := v.(map[string]any)
	if !ok {
		if b, isBool := v.(bool); isBool && b {
			return n, nil // true: anything
		}
		return nil, fmt.Errorf("schema: want an object, got %T", v)
	}
	switch t := m["type"].(type) {
	case string:
		n.types = map[string]bool{t: true}
	case []any:
		n.types = map[string]bool{}
		for _, x := range t {
			if s, ok := x.(string); ok {
				n.types[s] = true
			}
		}
	}
	addLit := func(x any) error {
		b, err := json.Marshal(x)
		if err != nil {
			return err
		}
		n.literals = append(n.literals, string(b))
		return nil
	}
	if c, ok := m["const"]; ok {
		if err := addLit(c); err != nil {
			return nil, err
		}
	}
	if e, ok := m["enum"].([]any); ok {
		for _, x := range e {
			if err := addLit(x); err != nil {
				return nil, err
			}
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		n.props = map[string]*schemaNode{}
		for k, pv := range props {
			c, err := compileNode(pv)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			n.props[k] = c
			n.order = append(n.order, k)
		}
		sort.Strings(n.order)
		n.closed = true // the runner writes only the properties it was given, unless additionalProperties says more
		if ap, ok := m["additionalProperties"].(bool); ok && ap {
			n.closed = false
		}
		if n.types == nil {
			n.types = map[string]bool{"object": true}
		}
	}
	n.required = map[string]bool{}
	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				n.required[s] = true
			}
		}
	}
	if it, ok := m["items"]; ok {
		c, err := compileNode(it)
		if err != nil {
			return nil, fmt.Errorf("items: %w", err)
		}
		n.items = c
	}
	if x, ok := m["minItems"].(float64); ok {
		n.minItems = int(x)
	}
	if x, ok := m["maxItems"].(float64); ok {
		n.maxItems = int(x)
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		if alts, ok := m[key].([]any); ok {
			for _, a := range alts {
				c, err := compileNode(a)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", key, err)
				}
				n.anyOf = append(n.anyOf, c)
			}
		}
	}
	return n, nil
}

// fit judges text as the start of a JSON value matching n.
func (n *schemaNode) fit(text string) engine.Fit {
	p := &jparser{s: text}
	p.ws()
	if p.eof() {
		return engine.FitPrefix
	}
	switch p.value(n) {
	case resMore:
		// only a number standing alone is complete while it could still go on; inside an object or array more must follow
		if c := text[len(text)-len(strings.TrimLeft(text, " \t\n\r"))]; p.openNumber && (c == '-' || (c >= '0' && c <= '9')) {
			return engine.FitComplete
		}
		return engine.FitPrefix
	case resBad:
		return engine.FitNo
	}
	p.ws()
	if !p.eof() {
		return engine.FitNo
	}
	return engine.FitDone
}

type result int

const (
	resOK   result = iota // a complete value
	resMore               // the text ran out inside the value: a valid start
	resBad                // can't match
)

// maxWS caps whitespace between tokens, so a model can't pad forever.
const maxWS = 4

type jparser struct {
	s string
	i int
	// openNumber: the text ended in a number that is complete as it stands (it could also go on)
	openNumber bool
}

func (p *jparser) eof() bool { return p.i >= len(p.s) }

// ws skips whitespace; false when there's more than maxWS of it.
func (p *jparser) ws() bool {
	n := 0
	for p.i < len(p.s) && strings.IndexByte(" \t\n\r", p.s[p.i]) >= 0 {
		p.i++
		n++
	}
	return n <= maxWS
}

// value matches one value against n.
func (p *jparser) value(n *schemaNode) result {
	if len(n.anyOf) > 0 {
		return p.alternatives(n.anyOf)
	}
	if n.literals != nil {
		return p.literal(n.literals)
	}
	if p.eof() {
		return resMore
	}
	any := len(n.types) == 0
	switch c := p.s[p.i]; {
	case c == '{' && (any || n.types["object"]):
		return p.object(n)
	case c == '[' && (any || n.types["array"]):
		return p.array(n)
	case c == '"' && (any || n.types["string"]):
		return p.str(nil)
	case (c == '-' || (c >= '0' && c <= '9')) && (any || n.types["number"] || n.types["integer"]):
		return p.number(!any && !n.types["number"])
	case c == 't' && (any || n.types["boolean"]):
		return p.literal([]string{"true"})
	case c == 'f' && (any || n.types["boolean"]):
		return p.literal([]string{"false"})
	case c == 'n' && (any || n.types["null"]):
		return p.literal([]string{"null"})
	}
	return resBad
}

// alternatives matches the first alternative that takes the whole value, else a valid start of any.
func (p *jparser) alternatives(alts []*schemaNode) result {
	start := p.i
	best, bestEnd, bestOpen := resBad, start, false
	for _, a := range alts {
		q := &jparser{s: p.s, i: start}
		switch r := q.value(a); r {
		case resOK:
			p.i, p.openNumber = q.i, false
			return resOK
		case resMore:
			if best != resMore {
				best, bestEnd, bestOpen = resMore, q.i, q.openNumber
			}
		}
	}
	p.i, p.openNumber = bestEnd, bestOpen
	return best
}

// literal matches one of lits (JSON texts) exactly; a prefix of one at the end of the text is a valid start.
func (p *jparser) literal(lits []string) result {
	rest := p.s[p.i:]
	matched := -1
	for _, l := range lits {
		if strings.HasPrefix(rest, l) && (matched < 0 || len(l) > matched) {
			matched = len(l)
		}
	}
	if matched >= 0 {
		// a longer literal could still be meant ("1" vs "10") if the text ends right here
		if p.i+matched == len(p.s) {
			for _, l := range lits {
				if len(l) > matched && strings.HasPrefix(l, rest) {
					p.i = len(p.s)
					return resMore
				}
			}
		}
		p.i += matched
		return resOK
	}
	for _, l := range lits {
		if strings.HasPrefix(l, rest) {
			p.i = len(p.s)
			return resMore
		}
	}
	return resBad
}

// str matches a JSON string. keys, if set, limits it to one of them (property names), a prefix at the end being a
// valid start; the key read is in p.key.
func (p *jparser) str(keys []string) result {
	if p.eof() {
		return resMore
	}
	if p.s[p.i] != '"' {
		return resBad
	}
	start := p.i
	p.i++
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == '"':
			p.i++
			if keys != nil {
				return p.keyMatch(keys, p.s[start:p.i], true)
			}
			return resOK
		case c == '\\':
			if p.i+1 >= len(p.s) {
				p.i = len(p.s)
				return resMore
			}
			e := p.s[p.i+1]
			if strings.IndexByte(`"\/bfnrt`, e) >= 0 {
				p.i += 2
				continue
			}
			if e != 'u' {
				return resBad
			}
			p.i += 2
			for k := 0; k < 4; k++ {
				if p.i >= len(p.s) {
					return resMore
				}
				if !isHex(p.s[p.i]) {
					return resBad
				}
				p.i++
			}
		case c < 0x20:
			return resBad
		default:
			p.i++
		}
	}
	if keys != nil {
		return p.keyMatch(keys, p.s[start:], false)
	}
	return resMore
}

// keyMatch checks a property name (JSON text, closed or not) against the allowed keys.
func (p *jparser) keyMatch(keys []string, got string, closed bool) result {
	for _, k := range keys {
		enc, _ := json.Marshal(k)
		if closed && got == string(enc) {
			return resOK
		}
		if !closed && strings.HasPrefix(string(enc), got) {
			return resMore
		}
	}
	return resBad
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// number matches a JSON number (an integer only, if integer).
func (p *jparser) number(integer bool) result {
	start := p.i
	if p.s[p.i] == '-' {
		p.i++
	}
	digits := func() int {
		n := 0
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
			n++
		}
		return n
	}
	if p.eof() {
		return resMore
	}
	if p.s[p.i] == '0' {
		p.i++
	} else if digits() == 0 {
		return resBad
	}
	complete := true
	if !integer && p.i < len(p.s) && p.s[p.i] == '.' {
		p.i++
		if digits() == 0 {
			complete = false
		}
	}
	if !integer && complete && p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		p.i++
		if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
			p.i++
		}
		if digits() == 0 {
			complete = false
		}
	}
	if p.eof() {
		// the text ends in the number: it may go on
		p.openNumber = complete && p.i > start
		return resMore
	}
	if !complete {
		return resBad
	}
	return resOK
}

// array matches [ value, ... ] with n's items and counts.
func (p *jparser) array(n *schemaNode) result {
	p.i++ // [
	items := n.items
	if items == nil {
		items = &schemaNode{maxItems: -1}
	}
	count := 0
	if !p.ws() {
		return resBad
	}
	if p.eof() {
		return resMore
	}
	if p.s[p.i] == ']' {
		if count < n.minItems {
			return resBad
		}
		p.i++
		return resOK
	}
	for {
		if n.maxItems >= 0 && count >= n.maxItems {
			return resBad
		}
		if r := p.value(items); r != resOK {
			return r
		}
		count++
		if !p.ws() {
			return resBad
		}
		if p.eof() {
			return resMore
		}
		switch p.s[p.i] {
		case ',':
			if n.maxItems >= 0 && count >= n.maxItems {
				return resBad
			}
			p.i++
			if !p.ws() {
				return resBad
			}
			if p.eof() {
				return resMore
			}
		case ']':
			if count < n.minItems {
				return resBad
			}
			p.i++
			return resOK
		default:
			return resBad
		}
	}
}

// object matches { "key": value, ... }: only n's properties when it lists them (each once), and the closing brace only
// once every required one is there.
func (p *jparser) object(n *schemaNode) result {
	p.i++ // {
	seen := map[string]bool{}
	if !p.ws() {
		return resBad
	}
	if p.eof() {
		return resMore
	}
	missing := func() bool {
		for r := range n.required {
			if !seen[r] {
				return true
			}
		}
		return false
	}
	if p.s[p.i] == '}' {
		if missing() {
			return resBad
		}
		p.i++
		return resOK
	}
	for {
		var keys []string
		if n.closed {
			for _, k := range n.order {
				if !seen[k] {
					keys = append(keys, k)
				}
			}
			if len(keys) == 0 {
				return resBad
			}
		}
		keyStart := p.i
		if r := p.str(keys); r != resOK {
			return r
		}
		var key string
		_ = json.Unmarshal([]byte(p.s[keyStart:p.i]), &key)
		if seen[key] {
			return resBad
		}
		seen[key] = true
		if !p.ws() {
			return resBad
		}
		if p.eof() {
			return resMore
		}
		if p.s[p.i] != ':' {
			return resBad
		}
		p.i++
		if !p.ws() {
			return resBad
		}
		sub := n.props[key]
		if sub == nil {
			sub = &schemaNode{maxItems: -1}
		}
		if r := p.value(sub); r != resOK {
			return r
		}
		if !p.ws() {
			return resBad
		}
		if p.eof() {
			return resMore
		}
		switch p.s[p.i] {
		case ',':
			if n.closed && len(seen) >= len(n.order) {
				return resBad // every property is written: only } may follow
			}
			p.i++
			if !p.ws() {
				return resBad
			}
			if p.eof() {
				return resMore
			}
		case '}':
			if missing() {
				return resBad
			}
			p.i++
			return resOK
		default:
			return resBad
		}
	}
}
