package bot_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
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
	sender := &fakeSender{
		quoted: bot.QuotedMessage{
			UserID:     12345,
			Nickname:   "张三",
			RawMessage: "被引用的消息",
		},
	}
	quoteGen := &capturingQuote{result: "base64-image"}
	p := bot.NewPipeline(bot.Options{
		Sender: sender,
		Quote:  quoteGen,
	})
	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID:        1,
		UserID:         2,
		Text:           "/q",
		ReplyMessageID: 99,
		RawMessage:     "/q",
	}); err != nil {
		t.Fatal(err)
	}
	if sender.requestedQuoteMessageID != 99 {
		t.Fatalf("requestedQuoteMessageID = %d", sender.requestedQuoteMessageID)
	}
	data, err := json.Marshal(quoteGen.payload)
	if err != nil {
		t.Fatal(err)
	}
	wantPayload := `[{"user_id":12345,"user_nickname":"张三","message":"被引用的消息"}]`
	if string(data) != wantPayload {
		t.Fatalf("payload = %s, want %s", data, wantPayload)
	}
	msg, ok := sender.lastMessage.(map[string]any)
	if !ok || msg["type"] != "image" {
		t.Fatalf("message = %#v", sender.lastMessage)
	}
	file := msg["data"].(map[string]any)["file"]
	if file != "base64://base64-image" {
		t.Fatalf("image file = %q", file)
	}
}

func TestPipelineQCommandConvertsCQImageToQuoteImageSegment(t *testing.T) {
	sender := &fakeSender{
		quoted: bot.QuotedMessage{
			UserID:     12345,
			Nickname:   "张三",
			RawMessage: `[CQ:image,file=BC7.png,sub_type=0,url=https://multimedia.nt.qq.com.cn/download?appid=1407&amp;fileid=abc,file_size=3617]`,
		},
	}
	quoteGen := &capturingQuote{result: "base64-image"}
	p := bot.NewPipeline(bot.Options{Sender: sender, Quote: quoteGen})

	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID:        1,
		UserID:         2,
		Text:           "/q",
		ReplyMessageID: 99,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(quoteGen.payload)
	if err != nil {
		t.Fatal(err)
	}
	wantPayload := `[{"user_id":12345,"user_nickname":"张三","message":[{"type":"image","url":"https://multimedia.nt.qq.com.cn/download?appid=1407\u0026fileid=abc"}]}]`
	if string(data) != wantPayload {
		t.Fatalf("payload = %s, want %s", data, wantPayload)
	}
}

func TestPipelineQCommandFiltersCQReplySegment(t *testing.T) {
	sender := &fakeSender{
		quoted: bot.QuotedMessage{
			UserID:     12345,
			Nickname:   "张三",
			RawMessage: "[CQ:reply,id=1773097336]/q",
		},
	}
	quoteGen := &capturingQuote{result: "base64-image"}
	p := bot.NewPipeline(bot.Options{Sender: sender, Quote: quoteGen})

	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID:        1,
		UserID:         2,
		Text:           "/q",
		ReplyMessageID: 99,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(quoteGen.payload)
	if err != nil {
		t.Fatal(err)
	}
	wantPayload := `[{"user_id":12345,"user_nickname":"张三","message":"/q"}]`
	if string(data) != wantPayload {
		t.Fatalf("payload = %s, want %s", data, wantPayload)
	}
}

func TestPipelineQCommandConvertsCQFaceToQuoteImageSegment(t *testing.T) {
	faceDir := t.TempDir()
	if err := os.WriteFile(faceDir+"/14.png", []byte("png-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JXH_QQ_FACE_DIR", faceDir)

	sender := &fakeSender{
		quoted: bot.QuotedMessage{
			UserID:     12345,
			Nickname:   "张三",
			RawMessage: "[CQ:face,id=14]",
		},
	}
	quoteGen := &capturingQuote{result: "base64-image"}
	p := bot.NewPipeline(bot.Options{Sender: sender, Quote: quoteGen})

	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID:        1,
		UserID:         2,
		Text:           "/q",
		ReplyMessageID: 99,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(quoteGen.payload)
	if err != nil {
		t.Fatal(err)
	}
	wantPayload := `[{"user_id":12345,"user_nickname":"张三","message":[{"type":"image","kind":"emoji","url":"data:image/png;base64,cG5nLWRhdGE="}]}]`
	if string(data) != wantPayload {
		t.Fatalf("payload = %s, want %s", data, wantPayload)
	}

	segments, ok := quoteGen.payload[0].Message.([]quote.MessageSegment)
	if !ok || len(segments) != 1 || segments[0].Type != "image" || !strings.HasPrefix(segments[0].URL, "data:image/png;base64,") {
		t.Fatalf("message = %#v", quoteGen.payload[0].Message)
	}
}

func TestPipelineQCommandConvertsCQEmojiToUnicodeText(t *testing.T) {
	sender := &fakeSender{
		quoted: bot.QuotedMessage{
			UserID:     12345,
			Nickname:   "张三",
			RawMessage: "[CQ:emoji,id=128512]",
		},
	}
	quoteGen := &capturingQuote{result: "base64-image"}
	p := bot.NewPipeline(bot.Options{Sender: sender, Quote: quoteGen})

	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID:        1,
		UserID:         2,
		Text:           "/q",
		ReplyMessageID: 99,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(quoteGen.payload)
	if err != nil {
		t.Fatal(err)
	}
	wantPayload := `[{"user_id":12345,"user_nickname":"张三","message":"😀"}]`
	if string(data) != wantPayload {
		t.Fatalf("payload = %s, want %s", data, wantPayload)
	}
}

func TestPipelineQCommandConvertsStructuredEmojiFileToImageSegment(t *testing.T) {
	sender := &fakeSender{
		quoted: bot.QuotedMessage{
			UserID:     12345,
			Nickname:   "张三",
			RawMessage: `[CQ:emoji,id=66,file=base64://cG5nLWRhdGE=]`,
			Message: []map[string]any{{
				"type": "emoji",
				"data": map[string]any{
					"id":   "66",
					"file": "base64://cG5nLWRhdGE=",
				},
			}},
		},
	}
	quoteGen := &capturingQuote{result: "base64-image"}
	p := bot.NewPipeline(bot.Options{Sender: sender, Quote: quoteGen})

	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID:        1,
		UserID:         2,
		Text:           "/q",
		ReplyMessageID: 99,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(quoteGen.payload)
	if err != nil {
		t.Fatal(err)
	}
	wantPayload := `[{"user_id":12345,"user_nickname":"张三","message":[{"type":"image","kind":"emoji","url":"data:image/png;base64,cG5nLWRhdGE="}]}]`
	if string(data) != wantPayload {
		t.Fatalf("payload = %s, want %s", data, wantPayload)
	}
}

func TestPipelineQCommandConvertsMFaceURLToImageSegment(t *testing.T) {
	sender := &fakeSender{
		quoted: bot.QuotedMessage{
			UserID:     12345,
			Nickname:   "张三",
			RawMessage: `[CQ:mface,emoji_id=123,url=https://gxh.vip.qq.com/club/item/parcel/item.png]`,
		},
	}
	quoteGen := &capturingQuote{result: "base64-image"}
	p := bot.NewPipeline(bot.Options{Sender: sender, Quote: quoteGen})

	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID:        1,
		UserID:         2,
		Text:           "/q",
		ReplyMessageID: 99,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(quoteGen.payload)
	if err != nil {
		t.Fatal(err)
	}
	wantPayload := `[{"user_id":12345,"user_nickname":"张三","message":[{"type":"image","kind":"sticker","url":"https://gxh.vip.qq.com/club/item/parcel/item.png"}]}]`
	if string(data) != wantPayload {
		t.Fatalf("payload = %s, want %s", data, wantPayload)
	}
}

func TestPipelineQCommandPrefersStructuredMFaceImageURL(t *testing.T) {
	sender := &fakeSender{
		quoted: bot.QuotedMessage{
			UserID:     12345,
			Nickname:   "张三",
			RawMessage: `[CQ:mface,emoji_id=123,summary=[惊讶]]`,
			Message: []map[string]any{{
				"type": "mface",
				"data": map[string]any{
					"emoji_id": "123",
					"url":      "https://gxh.vip.qq.com/club/item/parcel/structured.png",
					"summary":  "[惊讶]",
				},
			}},
		},
	}
	quoteGen := &capturingQuote{result: "base64-image"}
	p := bot.NewPipeline(bot.Options{Sender: sender, Quote: quoteGen})

	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID:        1,
		UserID:         2,
		Text:           "/q",
		ReplyMessageID: 99,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(quoteGen.payload)
	if err != nil {
		t.Fatal(err)
	}
	wantPayload := `[{"user_id":12345,"user_nickname":"张三","message":[{"type":"image","kind":"sticker","url":"https://gxh.vip.qq.com/club/item/parcel/structured.png"}]}]`
	if string(data) != wantPayload {
		t.Fatalf("payload = %s, want %s", data, wantPayload)
	}
}

func TestPipelineQCommandIgnoresUnusableStructuredMFaceFile(t *testing.T) {
	sender := &fakeSender{
		quoted: bot.QuotedMessage{
			UserID:     12345,
			Nickname:   "张三",
			RawMessage: `[CQ:mface,emoji_id=123,file=internal-name.png,summary=[惊讶]]`,
			Message: []map[string]any{{
				"type": "mface",
				"data": map[string]any{
					"emoji_id": "123",
					"file":     "internal-name.png",
					"summary":  "[惊讶]",
				},
			}},
		},
	}
	quoteGen := &capturingQuote{result: "base64-image"}
	p := bot.NewPipeline(bot.Options{Sender: sender, Quote: quoteGen})

	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID:        1,
		UserID:         2,
		Text:           "/q",
		ReplyMessageID: 99,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(quoteGen.payload)
	if err != nil {
		t.Fatal(err)
	}
	wantPayload := `[{"user_id":12345,"user_nickname":"张三","message":"[表情]"}]`
	if string(data) != wantPayload {
		t.Fatalf("payload = %s, want %s", data, wantPayload)
	}
}

func TestPipelineQCommandConvertsStructuredBase64ImageFileToDataURI(t *testing.T) {
	sender := &fakeSender{
		quoted: bot.QuotedMessage{
			UserID:     12345,
			Nickname:   "张三",
			RawMessage: `[CQ:image,file=base64://cG5nLWRhdGE=]`,
			Message: []map[string]any{{
				"type": "image",
				"data": map[string]any{
					"file": "base64://cG5nLWRhdGE=",
				},
			}},
		},
	}
	quoteGen := &capturingQuote{result: "base64-image"}
	p := bot.NewPipeline(bot.Options{Sender: sender, Quote: quoteGen})

	if err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{
		GroupID:        1,
		UserID:         2,
		Text:           "/q",
		ReplyMessageID: 99,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(quoteGen.payload)
	if err != nil {
		t.Fatal(err)
	}
	wantPayload := `[{"user_id":12345,"user_nickname":"张三","message":[{"type":"image","url":"data:image/png;base64,cG5nLWRhdGE="}]}]`
	if string(data) != wantPayload {
		t.Fatalf("payload = %s, want %s", data, wantPayload)
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
	lastGroupID             int64
	lastText                string
	lastMessage             any
	quoted                  bot.QuotedMessage
	requestedQuoteMessageID int64
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

func (f *fakeSender) GetQuoteMessage(ctx context.Context, messageID int64) (bot.QuotedMessage, error) {
	_ = ctx
	f.requestedQuoteMessageID = messageID
	return f.quoted, nil
}

type capturingQuote struct {
	result  string
	payload quote.Payload
}

func (q *capturingQuote) Generate(ctx context.Context, payload quote.Payload) (string, error) {
	_ = ctx
	q.payload = payload
	return q.result, nil
}

var _ = quote.Payload{}
