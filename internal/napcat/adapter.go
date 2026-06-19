package napcat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/zjutjh/jxh-go/internal/bot"
	napcatsdk "github.com/zjutjh/napcat-sdk"
	"github.com/zjutjh/napcat-sdk/api"
	"github.com/zjutjh/napcat-sdk/event"
	"github.com/zjutjh/napcat-sdk/message"
)

type Handler interface {
	HandleGroupMessage(ctx context.Context, msg bot.GroupMessage) error
	HandleGroupIncrease(ctx context.Context, groupID int64, userID int64) error
}

type Dedupe interface {
	SeenOrMark(key string) bool
}

type Server struct {
	Addr           string
	WSURL          string
	Token          string
	RequestTimeout time.Duration
	ReconnectDelay time.Duration
	Handler        Handler
	Dedupe         Dedupe
}

func (s Server) Serve(ctx context.Context) error {
	if s.WSURL != "" {
		return s.serveForwardWebSocket(ctx)
	}
	return napcatsdk.ServeReverseWebSocket(ctx, s.Addr, func(client *napcatsdk.Client) {
		s.consume(ctx, client)
	}, napcatsdk.WithToken(s.Token), napcatsdk.WithRequestTimeout(s.RequestTimeout))
}

func (s Server) serveForwardWebSocket(ctx context.Context) error {
	delay := s.ReconnectDelay
	if delay <= 0 {
		delay = 5 * time.Second
	}
	for {
		client, err := napcatsdk.DialWebSocket(ctx, s.WSURL, napcatsdk.WithToken(s.Token), napcatsdk.WithRequestTimeout(s.RequestTimeout))
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("connect napcat websocket failed: %v", err)
			if !sleepContext(ctx, delay) {
				return nil
			}
			continue
		}
		log.Printf("connected to napcat websocket: %s", s.WSURL)
		s.consume(ctx, client)
		_ = client.Close()
		if ctx.Err() != nil {
			return nil
		}
		log.Printf("napcat websocket disconnected, reconnecting in %s", delay)
		if !sleepContext(ctx, delay) {
			return nil
		}
	}
}

func (s Server) consume(ctx context.Context, client *napcatsdk.Client) {
	sender := SDKSender{client: client}
	if setter, ok := s.Handler.(interface{ SetSender(bot.Sender) }); ok {
		setter.SetSender(sender)
	}
	events := client.Events()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			if s.Dedupe != nil && s.Dedupe.SeenOrMark(eventKey(ev)) {
				continue
			}
			if err := s.handleEvent(ctx, client, ev); err != nil {
				log.Printf("handle napcat event failed: %v", err)
			}
		}
	}
}

func sleepContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s Server) handleEvent(ctx context.Context, client *napcatsdk.Client, ev event.Event) error {
	if s.Handler == nil {
		return nil
	}
	switch e := ev.(type) {
	case *event.GroupMessage:
		if err := markGroupMessageRead(ctx, client, e); err != nil {
			log.Printf("mark group message as read failed: %v", err)
		}
		return s.Handler.HandleGroupMessage(ctx, bot.GroupMessage{
			GroupID:        e.GroupID,
			UserID:         e.UserID,
			Text:           e.Message.Text(),
			RawMessage:     e.RawMessage,
			MessageID:      e.MessageID,
			ReplyMessageID: extractReplyID(e.Message),
			IsSelf:         e.UserID == e.SelfID(),
			AtUsers:        extractAtUsers(e.Message),
		})
	case *event.UnknownEvent:
		var notice struct {
			PostType   string `json:"post_type"`
			NoticeType string `json:"notice_type"`
			GroupID    int64  `json:"group_id"`
			UserID     int64  `json:"user_id"`
		}
		if err := json.Unmarshal(e.Raw(), &notice); err != nil {
			return nil
		}
		if notice.PostType == "notice" && notice.NoticeType == "group_increase" {
			return s.Handler.HandleGroupIncrease(ctx, notice.GroupID, notice.UserID)
		}
	}
	return nil
}

func markGroupMessageRead(ctx context.Context, client *napcatsdk.Client, e *event.GroupMessage) error {
	if client == nil || e == nil || e.MessageID == 0 {
		return nil
	}
	_, err := client.API().MarkGroupMsgAsRead(ctx, api.MarkGroupMsgAsReadRequest{
		GroupID:   strconv.FormatInt(e.GroupID, 10),
		MessageID: strconv.FormatInt(e.MessageID, 10),
	})
	return err
}

func eventKey(ev event.Event) string {
	switch e := ev.(type) {
	case *event.GroupMessage:
		return fmt.Sprintf("group-message:%d:%d:%d", e.GroupID, e.MessageID, e.Time())
	case *event.PrivateMessage:
		return fmt.Sprintf("private-message:%d:%d:%d", e.UserID, e.MessageID, e.Time())
	default:
		return fmt.Sprintf("%s:%d:%d", ev.PostType(), ev.SelfID(), ev.Time())
	}
}

func extractReplyID(chain message.Chain) int64 {
	for _, seg := range chain.OfType("reply") {
		raw := seg.String("id")
		id, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			return id
		}
	}
	return 0
}

type SDKSender struct {
	client *napcatsdk.Client
}

func NewSDKSender(client *napcatsdk.Client) SDKSender {
	return SDKSender{client: client}
}

func (s SDKSender) SendGroupText(ctx context.Context, groupID int64, text string) error {
	return s.SendGroupMessage(ctx, groupID, message.Text(text))
}

func (s SDKSender) SendGroupMessage(ctx context.Context, groupID int64, msg any) error {
	_, err := s.client.API().SendGroupMsg(ctx, api.SendGroupMsgRequest{
		GroupID: strconv.FormatInt(groupID, 10),
		Message: msg,
	})
	return err
}

func (s SDKSender) GetQuoteMessage(ctx context.Context, messageID int64) (bot.QuotedMessage, error) {
	var msg struct {
		UserID     any            `json:"user_id"`
		RawMessage string         `json:"raw_message"`
		Sender     map[string]any `json:"sender"`
		Message    any            `json:"message"`
	}
	if err := s.client.API().Call(ctx, string(api.ActionGetMsg), api.GetMsgRequest{MessageID: messageID}, &msg); err != nil {
		return bot.QuotedMessage{}, err
	}
	return bot.QuotedMessage{
		UserID:     anyInt64(msg.UserID),
		Nickname:   senderNickname(msg.Sender),
		RawMessage: msg.RawMessage,
		Message:    msg.Message,
	}, nil
}

func (s SDKSender) ResolveImage(ctx context.Context, file string) (string, error) {
	var data map[string]any
	if err := s.client.API().Call(ctx, "get_image", map[string]any{"file": file}, &data); err != nil {
		return "", err
	}
	if url := anyString(data["url"]); url != "" {
		return url, nil
	}
	return anyString(data["file"]), nil
}

func (s SDKSender) SetGroupBan(ctx context.Context, groupID, userID int64, duration time.Duration) error {
	_, err := s.client.API().SetGroupBan(ctx, api.SetGroupBanRequest{
		GroupID:  strconv.FormatInt(groupID, 10),
		UserID:   strconv.FormatInt(userID, 10),
		Duration: int64(duration.Seconds()),
	})
	return err
}

func (s SDKSender) SetRestart(ctx context.Context) error {
	_, err := s.client.API().SetRestart(ctx, api.SetRestartRequest{})
	return err
}

func extractAtUsers(chain message.Chain) []int64 {
	var out []int64
	for _, seg := range chain.OfType("at") {
		raw := seg.String("qq")
		if raw == "all" || raw == "" {
			continue
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			out = append(out, id)
		}
	}
	return out
}

func senderNickname(sender map[string]any) string {
	if card, ok := sender["card"].(string); ok && card != "" {
		return card
	}
	if nickname, ok := sender["nickname"].(string); ok {
		return nickname
	}
	return ""
}

func anyString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		return strconv.FormatInt(int64(value), 10)
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	default:
		return ""
	}
}

func anyInt64(v any) int64 {
	switch value := v.(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	case string:
		id, _ := strconv.ParseInt(value, 10, 64)
		return id
	default:
		return 0
	}
}
