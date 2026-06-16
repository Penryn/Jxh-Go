package vector

import (
	"context"
	"fmt"
	"strings"

	arkembed "github.com/cloudwego/eino-ext/components/embedding/ark"
	openaiembed "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"
)

type EinoEmbedder struct {
	embedder embedding.Embedder
}

type EinoEmbedderConfig struct {
	Provider   string
	BaseURL    string
	APIKey     string
	Model      string
	Dimensions int
}

func NewEinoEmbedder(ctx context.Context, cfg EinoEmbedderConfig) (*EinoEmbedder, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "openai"
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("eino embedding config is incomplete")
	}
	switch provider {
	case "openai":
		if cfg.BaseURL == "" || cfg.APIKey == "" {
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
	case "ark":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("eino embedding config is incomplete")
		}
		embedder, err := arkembed.NewEmbedder(ctx, &arkembed.EmbeddingConfig{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
			Model:   cfg.Model,
		})
		if err != nil {
			return nil, err
		}
		return &EinoEmbedder{embedder: embedder}, nil
	default:
		return nil, fmt.Errorf("unsupported eino embedding provider: %s", cfg.Provider)
	}
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
