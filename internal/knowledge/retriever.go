package knowledge

import (
	"context"
	"sort"
	"strings"
	"unicode"
)

type RetrievedDocument struct {
	Entry   Entry
	Score   float64
	Sources []string
}

type RetrievalOptions struct {
	Entries        []Entry
	ScoreThreshold float64
}

type RetrievalEngine struct {
	entries        []Entry
	scoreThreshold float64
}

type TextRetriever struct {
	engine *RetrievalEngine
}

func NewRetrievalEngine(opts RetrievalOptions) *RetrievalEngine {
	threshold := opts.ScoreThreshold
	if threshold < 0 {
		threshold = 0
	}
	return &RetrievalEngine{
		entries:        append([]Entry(nil), opts.Entries...),
		scoreThreshold: threshold,
	}
}

func NewTextRetriever(entries []Entry) *TextRetriever {
	return &TextRetriever{engine: NewRetrievalEngine(RetrievalOptions{Entries: entries})}
}

func (r *TextRetriever) Retrieve(ctx context.Context, query string, topK int) ([]RetrievedDocument, error) {
	if r == nil || r.engine == nil {
		return nil, nil
	}
	return r.engine.Retrieve(ctx, query, topK)
}

func (r *RetrievalEngine) Retrieve(ctx context.Context, query string, topK int) ([]RetrievedDocument, error) {
	_ = ctx
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if topK <= 0 {
		topK = 5
	}
	candidates := make(map[string]RetrievedDocument)
	for _, entry := range r.entries {
		if !entry.Enabled || !entry.AIEnabled {
			continue
		}
		for _, doc := range recallEntry(entry, query) {
			key := retrievalKey(doc.Entry)
			existing, ok := candidates[key]
			if !ok {
				candidates[key] = doc
				continue
			}
			if doc.Score > existing.Score {
				existing.Score = doc.Score
			}
			existing.Sources = uniqueStrings(append(existing.Sources, doc.Sources...))
			candidates[key] = existing
		}
	}
	docs := make([]RetrievedDocument, 0, len(candidates))
	for _, doc := range candidates {
		if doc.Score < r.scoreThreshold {
			continue
		}
		docs = append(docs, doc)
	}
	sort.SliceStable(docs, func(i, j int) bool {
		return docs[i].Score > docs[j].Score
	})
	if len(docs) > topK {
		docs = docs[:topK]
	}
	return docs, nil
}

func recallEntry(entry Entry, query string) []RetrievedDocument {
	var docs []RetrievedDocument
	if entry.Keyword == query {
		docs = append(docs, RetrievedDocument{Entry: entry, Score: 1, Sources: []string{"exact"}})
	}
	for _, alias := range entry.Aliases {
		if alias == query {
			docs = append(docs, RetrievedDocument{Entry: entry, Score: 0.9, Sources: []string{"exact"}})
			break
		}
	}
	if score := scoreText(entry, query); score > 0 {
		docs = append(docs, RetrievedDocument{Entry: entry, Score: score, Sources: []string{"text"}})
	}
	return docs
}

func scoreText(entry Entry, query string) float64 {
	var score float64
	haystack := strings.Join([]string{entry.Keyword, entry.Path, strings.Join(entry.Aliases, " "), entry.Answer, entry.Content}, "\n")
	for _, term := range queryTerms(query) {
		if strings.Contains(haystack, term) {
			score += 0.1
		}
	}
	if strings.Contains(haystack, query) {
		score += 0.2
	}
	if score > 0.8 {
		return 0.8
	}
	return score
}

func retrievalKey(entry Entry) string {
	if entry.SourceKey != "" {
		return entry.SourceKey
	}
	return normalizeKey(entry.Keyword)
}

func queryTerms(query string) []string {
	terms := strings.Fields(query)
	var cjk []rune
	flushCJK := func() {
		if len(cjk) < 2 {
			cjk = nil
			return
		}
		maxSize := 6
		if len(cjk) < maxSize {
			maxSize = len(cjk)
		}
		for size := 2; size <= maxSize; size++ {
			for start := 0; start+size <= len(cjk); start++ {
				terms = append(terms, string(cjk[start:start+size]))
			}
		}
		cjk = nil
	}
	for _, r := range query {
		if unicode.Is(unicode.Han, r) {
			cjk = append(cjk, r)
			continue
		}
		flushCJK()
	}
	flushCJK()
	return uniqueStrings(terms)
}
