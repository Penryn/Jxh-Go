package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zjutjh/jxh-go/internal/config"
)

func TestLoadAppliesDefaultsAndEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  debug: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JXH_ONEBOT_TOKEN", "token-from-env")
	t.Setenv("JXH_ONEBOT_WS_URL", "ws://napcat:3001")
	t.Setenv("JXH_MYSQL_DSN", "user:pass@tcp(localhost:3306)/jxh?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("JXH_AI_PROVIDER", "ark")
	t.Setenv("JXH_AI_API_KEY", "ai-key")
	t.Setenv("JXH_EMBEDDING_PROVIDER", "ark")
	t.Setenv("JXH_EMBEDDING_API_KEY", "embedding-key")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.OneBot.AccessToken != "token-from-env" {
		t.Fatalf("access token = %q", cfg.OneBot.AccessToken)
	}
	if cfg.OneBot.WSURL != "ws://napcat:3001" {
		t.Fatalf("onebot ws url = %q", cfg.OneBot.WSURL)
	}
	if cfg.OneBot.ReconnectIntervalSec != 5 {
		t.Fatalf("reconnect interval sec = %d", cfg.OneBot.ReconnectIntervalSec)
	}
	if cfg.Database.DSN != "user:pass@tcp(localhost:3306)/jxh?charset=utf8mb4&parseTime=True&loc=Local" {
		t.Fatalf("dsn = %q", cfg.Database.DSN)
	}
	if cfg.AI.APIKey != "ai-key" {
		t.Fatalf("ai api key = %q", cfg.AI.APIKey)
	}
	if cfg.AI.Provider != "ark" {
		t.Fatalf("ai provider = %q", cfg.AI.Provider)
	}
	if cfg.Embedding.Provider != "ark" {
		t.Fatalf("embedding provider = %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.APIKey != "embedding-key" {
		t.Fatalf("embedding api key = %q", cfg.Embedding.APIKey)
	}
	if cfg.Server.Addr != ":8080" {
		t.Fatalf("default server addr = %q", cfg.Server.Addr)
	}
	if cfg.Database.Host != "127.0.0.1" {
		t.Fatalf("default database host = %q", cfg.Database.Host)
	}
}

func TestDefaultAIProviderIsOpenAI(t *testing.T) {
	cfg := config.Default()
	if cfg.AI.Provider != "openai" {
		t.Fatalf("default ai provider = %q", cfg.AI.Provider)
	}
	if cfg.Embedding.Provider != "openai" {
		t.Fatalf("default embedding provider = %q", cfg.Embedding.Provider)
	}
}
