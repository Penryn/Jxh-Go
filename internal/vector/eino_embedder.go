package vector

import (
	"context"
	"fmt"

	openaiembed "github.com/cloudwego/eino-ext/components/embedding/openai"
)

type EinoEmbedder struct {
	embedder *openaiembed.Embedder
}

type EinoEmbedderConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	Dimensions int
}

func NewEinoEmbedder(ctx context.Context, cfg EinoEmbedderConfig) (*EinoEmbedder, error) {
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Model == "" {
		return nil, fmt.Errorf("eino embedding config is incomplete")
	}
	embedder, err := openaiembed.NewEmbedder(ctx, &openaiembed.EmbeddingConfig{
		BaseURL:    cfg.BaseURL,
		APIKey:     cfg.APIKey,
		Model:      cfg.Model,
		Dimensions: &cfg.Dimensions,
	})
	if err != nil {
		return nil, err
	}
	return &EinoEmbedder{embedder: embedder}, nil
}

func (e *EinoEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	vectors, err := e.embedder.EmbedStrings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}
	return vectors[0], nil
}
