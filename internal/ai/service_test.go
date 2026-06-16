package ai_test

import (
	"context"
	"strings"
	"testing"

	"github.com/zjutjh/jxh-go/internal/ai"
)

func TestServiceReturnsFixedMessageWhenNoDocuments(t *testing.T) {
	svc := ai.NewService(ai.Options{
		Retriever: ai.StaticRetriever{},
		Chat:      &ai.StaticChat{},
	})
	got, err := svc.Answer(context.Background(), "不存在的问题")
	if err != nil {
		t.Fatalf("Answer returned error: %v", err)
	}
	if got != "知识库里没有找到相关内容" {
		t.Fatalf("answer = %q", got)
	}
}

func TestServiceBuildsGroundedPrompt(t *testing.T) {
	chat := &ai.StaticChat{Response: "根据知识库：可以去杭州东站。"}
	svc := ai.NewService(ai.Options{
		Retriever: ai.StaticRetriever{Documents: []ai.Document{{
			ID:      "1",
			Content: "朝晖校区交通 / 火车站 / 杭州东站\n从朝晖去杭州东站路线",
			Metadata: map[string]string{
				"keyword": "%0011",
				"path":    "朝晖校区交通 / 火车站 / 杭州东站",
				"answer":  "从朝晖去杭州东站路线",
			},
		}}},
		Chat: chat,
	})
	got, err := svc.Answer(context.Background(), "朝晖去杭州东站怎么走")
	if err != nil {
		t.Fatalf("Answer returned error: %v", err)
	}
	if got == "" || !strings.Contains(chat.LastPrompt, "只能基于以下知识库内容回答") {
		t.Fatalf("bad answer/prompt: answer=%q prompt=%q", got, chat.LastPrompt)
	}
}
