package knowledge_test

import (
	"testing"

	"github.com/zjutjh/jxh-go/internal/knowledge"
)

func TestKeywordIndexMatchesKeywordAndAlias(t *testing.T) {
	idx := knowledge.NewKeywordIndex([]knowledge.Entry{{
		Keyword:    "选课",
		Aliases:    []string{"怎么选课"},
		Answer:     "选课说明",
		Enabled:    true,
		ExactReply: true,
	}})
	got, ok := idx.Lookup(" 怎么选课 ")
	if !ok {
		t.Fatal("alias did not match")
	}
	if got.Answer != "选课说明" {
		t.Fatalf("answer = %q", got.Answer)
	}
}
