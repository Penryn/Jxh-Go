package bot_test

import (
	"context"
	"testing"

	"github.com/zjutjh/jxh-go/internal/ai"
	"github.com/zjutjh/jxh-go/internal/bot"
	"github.com/zjutjh/jxh-go/internal/cache"
	"github.com/zjutjh/jxh-go/internal/commands"
	"github.com/zjutjh/jxh-go/internal/knowledge"
	"github.com/zjutjh/jxh-go/internal/quote"
)

func TestPipelineFallsBackToKeywordReply(t *testing.T) {
	sender := &fakeSender{}
	kc := cache.NewKnowledge()
	kc.Replace(knowledge.NewKeywordIndex([]knowledge.Entry{{Keyword: "精小弘", Answer: "我在！", Enabled: true, ExactReply: true}}))
	p := bot.NewPipeline(bot.Options{Knowledge: kc, Sender: sender})
	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{GroupID: 1, UserID: 2, Text: "精小弘"}); err != nil {
		t.Fatal(err)
	}
	if sender.lastText != "我在！" {
		t.Fatalf("lastText = %q", sender.lastText)
	}
}

func TestPipelineAdminCommandUsesHandler(t *testing.T) {
	sender := &fakeSender{}
	admins := commands.NewMemoryAdminStore()
	p := bot.NewPipeline(bot.Options{
		Sender: sender,
		Admin:  commands.NewAdminHandler(admins),
	})
	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID: 1,
		UserID:  1,
		Text:    "/admin 添加管理员",
		AtUsers: []int64{2},
		IsOwner: true,
	}); err != nil {
		t.Fatal(err)
	}
	if sender.lastText != "已添加管理员" {
		t.Fatalf("lastText = %q", sender.lastText)
	}
}

func TestPipelineQCommandGeneratesQuoteImage(t *testing.T) {
	sender := &fakeSender{}
	p := bot.NewPipeline(bot.Options{
		Sender: sender,
		Quote:  bot.StaticQuote{Result: "base64-image"},
	})
	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID:        1,
		UserID:         2,
		Text:           "/q",
		ReplyMessageID: 99,
		RawMessage:     "hello",
	}); err != nil {
		t.Fatal(err)
	}
	msg, ok := sender.lastMessage.(map[string]any)
	if !ok || msg["type"] != "image" {
		t.Fatalf("message = %#v", sender.lastMessage)
	}
}

func TestPipelineAICommandUsesService(t *testing.T) {
	sender := &fakeSender{}
	p := bot.NewPipeline(bot.Options{
		Sender: sender,
		AI: ai.NewService(ai.Options{
			Retriever: ai.StaticRetriever{Documents: []ai.Document{{ID: "1", Content: "选课说明"}}},
			Chat:      &ai.StaticChat{Response: "这是 AI 回答"},
		}),
	})
	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{GroupID: 1, UserID: 2, Text: "/ai 怎么选课"}); err != nil {
		t.Fatal(err)
	}
	if sender.lastText != "这是 AI 回答" {
		t.Fatalf("lastText = %q", sender.lastText)
	}
}

type fakeSender struct {
	lastGroupID int64
	lastText    string
	lastMessage any
}

func (f *fakeSender) SendGroupText(ctx context.Context, groupID int64, text string) error {
	_ = ctx
	f.lastGroupID = groupID
	f.lastText = text
	return nil
}

func (f *fakeSender) SendGroupMessage(ctx context.Context, groupID int64, message any) error {
	_ = ctx
	f.lastGroupID = groupID
	f.lastMessage = message
	f.lastText = ""
	if text, ok := message.(string); ok {
		f.lastText = text
	}
	return nil
}

var _ = quote.Payload{}
