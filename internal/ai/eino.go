package ai

import (
	"context"
	"fmt"
	"strings"

	arkmodel "github.com/cloudwego/eino-ext/components/model/ark"
	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type EinoChat struct {
	model model.ToolCallingChatModel
}

type EinoChatConfig struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
}

func NewEinoChat(ctx context.Context, cfg EinoChatConfig) (*EinoChat, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "openai"
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("eino chat config is incomplete")
	}
	switch provider {
	case "openai":
		if cfg.BaseURL == "" || cfg.APIKey == "" {
			return nil, fmt.Errorf("eino chat config is incomplete")
		}
		chatModel, err := openaimodel.NewChatModel(ctx, &openaimodel.ChatModelConfig{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
			Model:   cfg.Model,
		})
		if err != nil {
			return nil, err
		}
		return &EinoChat{model: chatModel}, nil
	case "ark":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("eino chat config is incomplete")
		}
		chatModel, err := arkmodel.NewChatModel(ctx, &arkmodel.ChatModelConfig{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
			Model:   cfg.Model,
		})
		if err != nil {
			return nil, err
		}
		return &EinoChat{model: chatModel}, nil
	default:
		return nil, fmt.Errorf("unsupported eino chat provider: %s", cfg.Provider)
	}
}

func (c *EinoChat) Generate(ctx context.Context, prompt string) (string, error) {
	msg, err := c.model.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
	if err != nil {
		return "", err
	}
	if msg == nil {
		return "", nil
	}
	return msg.Content, nil
}
