package knowledge_test

import (
	"context"
	"testing"

	"github.com/zjutjh/jxh-go/internal/knowledge"
)

func TestTextRetrieverMatchesChineseNaturalLanguageQuestion(t *testing.T) {
	entries, _ := knowledge.ParseRows([][]string{
		{"%000", "杭州城区交通\n\n%001 火车站", ""},
		{"%001", "请选择\n\n%0011 杭州站\n%0012 杭州东站", ""},
		{"%0011", "杭州站是杭州城区客运火车站。", ""},
		{"%0012", "杭州东站是杭州城区客运火车站。", ""},
	})
	retriever := knowledge.NewTextRetriever(entries)

	docs, err := retriever.Retrieve(context.Background(), "杭州城区有几个客运火车站", 5)
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected Chinese natural language question to retrieve train station knowledge")
	}
}
