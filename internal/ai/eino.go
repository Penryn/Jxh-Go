package ai

import (
	"context"
	"fmt"

	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

type EinoChat struct {
	model *openaimodel.ChatModel
}

type EinoChatConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

func NewEinoChat(ctx context.Context, cfg EinoChatConfig) (*EinoChat, error) {
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Model == "" {
		return nil, fmt.Errorf("eino chat config is incomplete")
	}
	model, err := openaimodel.NewChatModel(ctx, &openaimodel.ChatModelConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, err
	}
	return &EinoChat{model: model}, nil
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
