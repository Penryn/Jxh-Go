package knowledge

import "strings"

type KeywordIndex struct {
	entries map[string]Entry
}

func NewKeywordIndex(entries []Entry) *KeywordIndex {
	idx := &KeywordIndex{entries: make(map[string]Entry)}
	for _, entry := range entries {
		if !entry.Enabled || !entry.ExactReply {
			continue
		}
		idx.entries[normalizeLookup(entry.Keyword)] = entry
		for _, alias := range entry.Aliases {
			idx.entries[normalizeLookup(alias)] = entry
		}
	}
	return idx
}

func (i *KeywordIndex) Lookup(message string) (Entry, bool) {
	if i == nil {
		return Entry{}, false
	}
	entry, ok := i.entries[normalizeLookup(message)]
	return entry, ok
}

func normalizeLookup(value string) string {
	return strings.TrimSpace(value)
}
