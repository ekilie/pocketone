package kvv1

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"
)

// Prefix starts the _params id of every value in the store: name "storytime/settings" is row "kv:storytime/settings".
const Prefix = "kv:"

// MaxNameLength is the longest name.
const MaxNameLength = 128

// MaxValueBytes is the largest value, as JSON.
const MaxValueBytes = 1 << 20

// PrivatePrefix starts a private name ("_llm/settings"): Go code reads and writes it through these helpers, but the
// wire API (HTTP, the CLI, MCP) and page code's $kv never reach or list it. Keep secrets (API keys, signing keys) there.
const PrivatePrefix = "_"

// IsPrivate says whether name is private (starts with [PrivatePrefix]).
func IsPrivate(name string) bool { return strings.HasPrefix(name, PrivatePrefix) }

// ValidName returns [ErrInvalidName] unless name can name a value: after an optional [PrivatePrefix], a letter or digit,
// then letters, digits and _ - . : / , at most [MaxNameLength] in all.
func ValidName(name string) error {
	rest := strings.TrimPrefix(name, PrivatePrefix)
	if rest == "" || len(name) > MaxNameLength || !alnum(rest[0]) {
		return ErrInvalidName
	}
	if !validChars(name) {
		return ErrInvalidName
	}
	return nil
}

// validPrefix: a list prefix uses the names' characters and may be empty.
func validPrefix(prefix string) error {
	if len(prefix) > MaxNameLength || !validChars(prefix) {
		return ErrInvalidName
	}
	return nil
}

func validChars(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !alnum(c) && !strings.ContainsRune("_-.:/", rune(c)) {
			return false
		}
	}
	return true
}

func alnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// ParamID is the _params id that holds name's value.
func ParamID(name string) string { return Prefix + name }

// Lookup returns name's stored value; found is false when it has none.
func Lookup(app core.App, name string) (e Entry, found bool, err error) {
	if err := ValidName(name); err != nil {
		return Entry{}, false, err
	}
	p := &core.Param{}
	if err := app.ModelQuery(p).Model(ParamID(name), p); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, false, nil
		}
		return Entry{}, false, err
	}
	return entryOf(p), true, nil
}

// Get decodes name's value into dst (as json.Unmarshal). found is false, and dst untouched, when it has none.
//
//	var s Settings
//	if _, err := kvv1.Get(app, "storytime/settings", &s); err != nil { ... }
func Get(app core.App, name string, dst any) (found bool, err error) {
	e, found, err := Lookup(app, name)
	if err != nil || !found {
		return false, err
	}
	return true, json.Unmarshal(e.Value, dst)
}

// Set stores v (as json.Marshal) under name, replacing any value it had.
func Set(app core.App, name string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = SetRaw(app, name, raw)
	return err
}

// SetRaw stores raw JSON under name and returns the stored entry.
func SetRaw(app core.App, name string, raw json.RawMessage) (Entry, error) {
	if err := ValidName(name); err != nil {
		return Entry{}, err
	}
	if len(raw) == 0 || len(raw) > MaxValueBytes || !json.Valid(raw) {
		return Entry{}, ErrInvalidValue
	}
	var e Entry
	err := app.RunInTransaction(func(tx core.App) error {
		p := &core.Param{}
		if err := tx.ModelQuery(p).Model(ParamID(name), p); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			p = &core.Param{}
			p.Id = ParamID(name)
			p.MarkAsNew()
		}
		now := types.NowDateTime()
		if p.IsNew() {
			p.Created = now
		}
		p.Updated = now
		p.Value = types.JSONRaw(compact(raw))
		if err := tx.Save(p); err != nil {
			return err
		}
		e = entryOf(p)
		return nil
	})
	return e, err
}

// Delete removes name's value. Deleting a name with no value is not an error.
func Delete(app core.App, name string) error {
	if err := ValidName(name); err != nil {
		return err
	}
	_, err := app.NonconcurrentDB().Delete((&core.Param{}).TableName(), dbx.HashExp{"id": ParamID(name)}).Execute()
	return err
}

// List returns the values whose names start with prefix (every value when it's empty), sorted by name.
func List(app core.App, prefix string) ([]Entry, error) {
	if err := validPrefix(prefix); err != nil {
		return nil, err
	}
	id := ParamID(prefix)
	var ps []*core.Param
	err := app.ModelQuery(&core.Param{}).
		AndWhere(dbx.NewExp("substr([[id]], 1, {:n}) = {:p}", dbx.Params{"n": len(id), "p": id})).
		All(&ps)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(ps))
	for _, p := range ps {
		out = append(out, entryOf(p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// MoveParam moves the _params row fromID (an extension's own row from before kv/v1) to name, once: when name has no
// value yet, it gets the row's value and times, and the row is deleted. When name already has one, a leftover row is
// just deleted. moved says whether a value was moved. PocketBase's own rows and the store's rows can't be moved.
func MoveParam(app core.App, fromID, name string) (moved bool, err error) {
	if err := ValidName(name); err != nil {
		return false, err
	}
	if fromID == "" || fromID == "settings" || strings.HasPrefix(fromID, Prefix) {
		return false, errors.New("kv/v1: MoveParam: can't move _params row " + fromID)
	}
	err = app.RunInTransaction(func(tx core.App) error {
		old := &core.Param{}
		if err := tx.ModelQuery(old).Model(fromID, old); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		if _, found, err := Lookup(tx, name); err != nil {
			return err
		} else if !found {
			p := &core.Param{Created: old.Created, Updated: old.Updated, Value: old.Value}
			p.Id = ParamID(name)
			p.MarkAsNew()
			if err := tx.Save(p); err != nil {
				return err
			}
			moved = true
		}
		return tx.Delete(old)
	})
	return moved, err
}

// MoveParamOnce is [MoveParam] at most once per app and row, for a load path that runs often: after the first call that
// succeeds, later ones return at once.
func MoveParamOnce(app core.App, fromID, name string) error {
	m := app.Store().GetOrSet("kv/v1.move:"+fromID, func() any { return &moveOnce{} }).(*moveOnce)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.done {
		return nil
	}
	if _, err := MoveParam(app, fromID, name); err != nil {
		return err
	}
	m.done = true
	return nil
}

type moveOnce struct {
	mu   sync.Mutex
	done bool
}

func entryOf(p *core.Param) Entry {
	return Entry{
		Name:    strings.TrimPrefix(p.Id, Prefix),
		Value:   json.RawMessage(p.Value),
		Created: p.Created.Time(),
		Updated: p.Updated.Time(),
	}
}

func compact(raw json.RawMessage) []byte {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return raw
	}
	return b.Bytes()
}
