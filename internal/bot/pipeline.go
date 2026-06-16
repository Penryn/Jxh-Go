package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zjutjh/jxh-go/internal/ai"
	"github.com/zjutjh/jxh-go/internal/cache"
	"github.com/zjutjh/jxh-go/internal/commands"
	"github.com/zjutjh/jxh-go/internal/quote"
)

type Sender interface {
	SendGroupText(ctx context.Context, groupID int64, text string) error
	SendGroupMessage(ctx context.Context, groupID int64, message any) error
}

type Reloader interface {
	Reload(ctx context.Context) error
}

type Blacklist interface {
	IsBlacklisted(ctx context.Context, userID int64) (bool, error)
}

type QuoteGenerator interface {
	Generate(ctx context.Context, payload quote.Payload) (string, error)
}

type Moderator interface {
	SetGroupBan(ctx context.Context, groupID, userID int64, duration time.Duration) error
	SetRestart(ctx context.Context) error
}

type Options struct {
	Knowledge *cache.Knowledge
	Sender    Sender
	AI        *ai.Service
	Reloader  Reloader
	Blacklist Blacklist
	Admin     *commands.AdminHandler
	Quote     QuoteGenerator
}

type Pipeline struct {
	mu        sync.RWMutex
	knowledge *cache.Knowledge
	sender    Sender
	ai        *ai.Service
	reloader  Reloader
	blacklist Blacklist
	admin     *commands.AdminHandler
	quote     QuoteGenerator
}

func (p *Pipeline) SetSender(sender Sender) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sender = sender
}

type GroupMessage struct {
	GroupID        int64
	UserID         int64
	Text           string
	RawMessage     string
	MessageID      int64
	ReplyMessageID int64
	IsSelf         bool
	IsOwner        bool
	AtUsers        []int64
}

func NewPipeline(opts Options) *Pipeline {
	return &Pipeline{
		knowledge: opts.Knowledge,
		sender:    opts.Sender,
		ai:        opts.AI,
		reloader:  opts.Reloader,
		blacklist: opts.Blacklist,
		admin:     opts.Admin,
		quote:     opts.Quote,
	}
}

func (p *Pipeline) HandleGroupMessage(ctx context.Context, msg GroupMessage) error {
	sender := p.currentSender()
	if sender == nil || msg.IsSelf {
		return nil
	}
	if p.blacklist != nil {
		blocked, err := p.blacklist.IsBlacklisted(ctx, msg.UserID)
		if err != nil {
			return err
		}
		if blocked {
			return nil
		}
	}
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return nil
	}
	switch {
	case text == "/test":
		return sender.SendGroupText(ctx, msg.GroupID, "ddd")
	case text == "/reload":
		if p.reloader != nil {
			if err := p.reloader.Reload(ctx); err != nil {
				return err
			}
		}
		return sender.SendGroupText(ctx, msg.GroupID, "重载成功")
	case text == "/q":
		if p.quote == nil {
			return sender.SendGroupText(ctx, msg.GroupID, "引用图服务未初始化")
		}
		if msg.ReplyMessageID == 0 {
			return sender.SendGroupText(ctx, msg.GroupID, "请回复一条消息后使用 /q")
		}
		image, err := p.quote.Generate(ctx, quote.Payload{MessageID: msg.ReplyMessageID, RawMessage: msg.RawMessage})
		if err != nil {
			return err
		}
		return sender.SendGroupMessage(ctx, msg.GroupID, map[string]any{"type": "image", "data": map[string]any{"file": image}})
	case strings.HasPrefix(text, "/ai"):
		question := strings.TrimSpace(strings.TrimPrefix(text, "/ai"))
		if p.ai == nil {
			return sender.SendGroupText(ctx, msg.GroupID, ai.EmptyKnowledgeAnswer)
		}
		answer, err := p.ai.Answer(ctx, question)
		if err != nil {
			return err
		}
		return sender.SendGroupText(ctx, msg.GroupID, answer)
	case strings.HasPrefix(text, "/admin"):
		adminText := strings.TrimSpace(strings.TrimPrefix(text, "/admin"))
		if adminText == "restart" {
			moderator, ok := sender.(Moderator)
			if !ok {
				return sender.SendGroupText(ctx, msg.GroupID, "NapCat 管理接口未初始化")
			}
			if err := moderator.SetRestart(ctx); err != nil {
				return err
			}
			return sender.SendGroupText(ctx, msg.GroupID, "已请求重启 NapCat")
		}
		if strings.HasPrefix(adminText, "ban ") {
			moderator, ok := sender.(Moderator)
			if !ok {
				return sender.SendGroupText(ctx, msg.GroupID, "NapCat 管理接口未初始化")
			}
			if len(msg.AtUsers) == 0 {
				return sender.SendGroupText(ctx, msg.GroupID, "请 @ 要禁言的用户")
			}
			duration, err := parseBanDuration(strings.TrimSpace(strings.TrimPrefix(adminText, "ban ")))
			if err != nil {
				return sender.SendGroupText(ctx, msg.GroupID, "禁言时间格式不正确")
			}
			if err := moderator.SetGroupBan(ctx, msg.GroupID, msg.AtUsers[0], duration); err != nil {
				return err
			}
			return sender.SendGroupText(ctx, msg.GroupID, "已禁言")
		}
		if p.admin == nil {
			return sender.SendGroupText(ctx, msg.GroupID, "管理命令未初始化")
		}
		resp, err := p.admin.Handle(ctx, commands.AdminInput{
			ActorID: msg.UserID,
			Text:    adminText,
			AtUsers: msg.AtUsers,
			IsOwner: msg.IsOwner,
		})
		if err != nil {
			return err
		}
		return sender.SendGroupText(ctx, msg.GroupID, resp)
	}
	if p.knowledge != nil {
		if entry, ok := p.knowledge.Lookup(text); ok {
			return sender.SendGroupText(ctx, msg.GroupID, entry.Answer)
		}
	}
	return nil
}

func parseBanDuration(raw string) (time.Duration, error) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty duration")
	}
	if d, err := time.ParseDuration(fields[0]); err == nil {
		return d, nil
	}
	seconds, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds) * time.Second, nil
}

func (p *Pipeline) HandleGroupIncrease(ctx context.Context, groupID int64, userID int64) error {
	sender := p.currentSender()
	if sender == nil {
		return nil
	}
	message := []any{
		map[string]any{"type": "at", "data": map[string]any{"qq": userID}},
		map[string]any{"type": "text", "data": map[string]any{"text": "欢迎来到浙江工业大学，精弘网络欢迎各位的到来！\n输入 菜单 获取精小弘机器人的菜单 哦！\n请及时修改群名片\n格式如下：专业/大类+姓名"}},
	}
	return sender.SendGroupMessage(ctx, groupID, message)
}

func (p *Pipeline) SendGroupText(ctx context.Context, groupID int64, text string) error {
	sender := p.currentSender()
	if sender == nil {
		return nil
	}
	return sender.SendGroupText(ctx, groupID, text)
}

func (p *Pipeline) currentSender() Sender {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sender
}

type StaticQuote struct {
	Result string
	Err    error
}

func (q StaticQuote) Generate(ctx context.Context, payload quote.Payload) (string, error) {
	_ = ctx
	_ = payload
	if q.Err != nil {
		return "", q.Err
	}
	if q.Result == "" {
		return "", fmt.Errorf("empty quote result")
	}
	return q.Result, nil
}
