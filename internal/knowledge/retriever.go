package knowledge

import (
	"context"
	"sort"
	"strings"
)

type RetrievedDocument struct {
	Entry   Entry
	Score   float64
	Sources []string
}

type TextRetriever struct {
	entries []Entry
}

func NewTextRetriever(entries []Entry) *TextRetriever {
	return &TextRetriever{entries: entries}
}

func (r *TextRetriever) Retrieve(ctx context.Context, query string, topK int) ([]RetrievedDocument, error) {
	_ = ctx
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if topK <= 0 {
		topK = 5
	}
	var docs []RetrievedDocument
	for _, entry := range r.entries {
		if !entry.Enabled || !entry.AIEnabled {
			continue
		}
		score, sources := scoreEntry(entry, query)
		if score <= 0 {
			continue
		}
		docs = append(docs, RetrievedDocument{Entry: entry, Score: score, Sources: sources})
	}
	sort.SliceStable(docs, func(i, j int) bool {
		return docs[i].Score > docs[j].Score
	})
	if len(docs) > topK {
		docs = docs[:topK]
	}
	return docs, nil
}

func scoreEntry(entry Entry, query string) (float64, []string) {
	var score float64
	var sources []string
	if entry.Keyword == query {
		score += 100
		sources = append(sources, "exact")
	}
	for _, alias := range entry.Aliases {
		if alias == query {
			score += 90
			sources = append(sources, "exact")
			break
		}
	}
	haystack := strings.Join([]string{entry.Keyword, entry.Path, strings.Join(entry.Aliases, " "), entry.Answer, entry.Content}, "\n")
	for _, term := range strings.Fields(query) {
		if strings.Contains(haystack, term) {
			score += 10
			sources = append(sources, "like")
		}
	}
	if strings.Contains(haystack, query) {
		score += 20
		sources = append(sources, "fulltext")
	}
	return score, uniqueStrings(sources)
}
