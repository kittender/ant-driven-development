package core

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// Class separates what any permitted ageant may say (request) from what
// only the Ecosystem or a human may say (verdict).
type Class string

const (
	Request Class = "request"
	Verdict Class = "verdict"
)

// Entry is one line of the signal dictionary.
type Entry struct {
	Alias   string `json:"alias"`   // readable name used in settings, e.g. "needs_test"
	Emoji   string `json:"emoji"`   // the single code point, e.g. "🧪"
	Class   Class  `json:"class"`   // request | verdict
	Meaning string `json:"meaning"` // one sentence, shown to ageants
}

// Dictionary is the closed set of signals. It is immutable once built.
type Dictionary struct {
	entries []Entry
	byEmoji map[string]Entry
	byAlias map[string]Entry
}

// NewDictionary validates entries and builds the lookup maps.
func NewDictionary(entries []Entry) (*Dictionary, error) {
	d := &Dictionary{
		entries: append([]Entry(nil), entries...),
		byEmoji: make(map[string]Entry, len(entries)),
		byAlias: make(map[string]Entry, len(entries)),
	}
	for i, e := range entries {
		if e.Alias == "" {
			return nil, fmt.Errorf("entry %d: empty alias", i)
		}
		if err := ValidSignalEmoji(e.Emoji); err != nil {
			return nil, fmt.Errorf("entry %q: %w", e.Alias, err)
		}
		if e.Class != Request && e.Class != Verdict {
			return nil, fmt.Errorf("entry %q: class %q is not request or verdict", e.Alias, e.Class)
		}
		if _, dup := d.byEmoji[e.Emoji]; dup {
			return nil, fmt.Errorf("entry %q: emoji %s used twice", e.Alias, e.Emoji)
		}
		if _, dup := d.byAlias[e.Alias]; dup {
			return nil, fmt.Errorf("alias %q used twice", e.Alias)
		}
		d.byEmoji[e.Emoji] = e
		d.byAlias[e.Alias] = e
	}
	return d, nil
}

// LoadDictionary reads a JSON array of entries. Unknown fields are an error:
// the harness must refuse settings it does not recognise.
func LoadDictionary(r io.Reader) (*Dictionary, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var entries []Entry
	if err := dec.Decode(&entries); err != nil {
		return nil, fmt.Errorf("dictionary: %w", err)
	}
	return NewDictionary(entries)
}

// Lookup finds an entry by emoji (after normalisation by the caller).
func (d *Dictionary) Lookup(emoji string) (Entry, bool) {
	e, ok := d.byEmoji[emoji]
	return e, ok
}

// ByAlias finds an entry by its alias.
func (d *Dictionary) ByAlias(alias string) (Entry, bool) {
	e, ok := d.byAlias[alias]
	return e, ok
}

// Spell rewrites every dictionary emoji in s as its alias, so a text can be
// shown to an ageant in words ("✅ 🧪🧪" becomes "passed needs_test needs_test").
// Other characters, including emoji not in the dictionary, are kept.
func (d *Dictionary) Spell(s string) string {
	var b strings.Builder
	prevAlias := false
	for _, r := range s {
		if r == variationSelector16 {
			continue
		}
		if e, ok := d.byEmoji[string(r)]; ok {
			if prevAlias {
				b.WriteByte(' ')
			}
			b.WriteString(e.Alias)
			prevAlias = true
			continue
		}
		prevAlias = false
		b.WriteRune(r)
	}
	return b.String()
}

// Annotate writes each dictionary emoji in s followed by its alias, so the
// reader gets both ("✅ 🧪" becomes "✅ passed 🧪 needs_test").
func (d *Dictionary) Annotate(s string) string {
	var b strings.Builder
	afterAlias := false
	for _, r := range s {
		if r == variationSelector16 {
			continue
		}
		if afterAlias && !unicode.IsSpace(r) {
			b.WriteByte(' ') // keep "🧪🧪" readable: "🧪 needs_test 🧪 needs_test"
		}
		b.WriteRune(r)
		afterAlias = false
		if e, ok := d.byEmoji[string(r)]; ok {
			b.WriteString(" " + e.Alias)
			afterAlias = true
		}
	}
	return b.String()
}

// Entries returns a copy of all entries in file order.
func (d *Dictionary) Entries() []Entry {
	return append([]Entry(nil), d.entries...)
}
