# Jxh Go NapCat Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single-instance Go replacement for `MangoGovo/qqbot-JXH` that preserves the current bot features and connects to NapCat through `github.com/zjutjh/napcat-sdk`.

**Architecture:** NapCat handles QQ login and OneBot 11 protocol. The Go bot runs one process, receives NapCat reverse WebSocket events through `napcat-sdk`, routes group messages through deterministic command/reply logic, persists state in MySQL with GORM, and calls the existing quote service for `/q`. No `/ai`, MCP, admin UI, long-term memory, or multi-instance coordination is implemented.

**Tech Stack:** Go 1.25+, `github.com/zjutjh/napcat-sdk`, `go.uber.org/zap`, `gopkg.in/yaml.v3`, `gorm.io/gorm`, `gorm.io/driver/mysql`, `github.com/xuri/excelize/v2`, `github.com/robfig/cron/v3`, `github.com/jellydator/ttlcache/v3`, MySQL 8.4, Docker Compose.

---

## File Structure

Create these files:

- `go.mod`: Go module and dependency declarations.
- `.gitignore`: Ignore OS files, build output, local config, and data.
- `cmd/bot/main.go`: Application entrypoint.
- `config/config.example.yaml`: Example runtime configuration.
- `internal/config/config.go`: YAML and environment configuration loader.
- `internal/logger/logger.go`: zap logger setup.
- `internal/domain/message.go`: Internal group message and message segment types.
- `internal/domain/result.go`: Command result and outbound message types.
- `internal/napcat/adapter.go`: `napcat-sdk` reverse WebSocket startup and event adaptation.
- `internal/napcat/actions.go`: SDK-backed action client for send/get/ban/restart.
- `internal/bot/pipeline.go`: Group message pipeline.
- `internal/bot/permissions.go`: Admin and blacklist decisions.
- `internal/commands/router.go`: Command matching and dispatch.
- `internal/commands/admin.go`: `/admin` command implementation.
- `internal/commands/reload.go`: `/reload` command implementation.
- `internal/commands/quote.go`: `/q` command implementation.
- `internal/commands/test.go`: `/test` command implementation.
- `internal/reply/service.go`: Reply rule cache and exact-match lookup.
- `internal/reply/wps.go`: WPS download and Excel parsing.
- `internal/storage/models.go`: GORM models.
- `internal/storage/db.go`: MySQL connection and migration.
- `internal/storage/admins.go`: Admin repository.
- `internal/storage/blacklist.go`: Blacklist repository.
- `internal/storage/replies.go`: Reply rule repository.
- `internal/storage/jobs.go`: Scheduled job repository.
- `internal/scheduler/scheduler.go`: cron-backed scheduled message runner.
- `internal/quote/client.go`: Quote service HTTP client.
- `Dockerfile`: Multi-stage Go build.
- `compose.yaml`: MySQL, bot, NapCat, quote services.
- `README.md`: Local runbook.

Create these tests:

- `internal/config/config_test.go`
- `internal/napcat/adapter_test.go`
- `internal/bot/pipeline_test.go`
- `internal/commands/router_test.go`
- `internal/commands/admin_test.go`
- `internal/commands/quote_test.go`
- `internal/reply/wps_test.go`
- `internal/storage/storage_test.go`
- `internal/scheduler/scheduler_test.go`
- `internal/quote/client_test.go`

---

### Task 1: Initialize Module, Ignore Rules, Config, And Logger

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `config/config.example.yaml`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/logger/logger.go`

- [ ] **Step 1: Write the failing config tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadReadsYAMLAndEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte(`
app:
  debug: true
  log_level: "debug"
  timezone: "Asia/Shanghai"
server:
  addr: ":8080"
  onebot_path: "/onebot/v11/ws"
onebot:
  access_token: "file-token"
  api_timeout_sec: 30
wps_excel:
  share_url: "https://example.com/sheet"
  sid: "file-sid"
  sheet: "release"
  cache_file: "./data/cache/replies.xlsx"
database:
  host: "mysql"
  port: 3306
  user: "jxh"
  password: "file-password"
  name: "jxh_bot"
  charset: "utf8mb4"
  parse_time: true
  loc: "Local"
quote:
  base_url: "http://quote:5000"
  timeout_sec: 10
scheduler:
  timezone: "Asia/Shanghai"
debug:
  enable_test_command: true
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JXH_ONEBOT_TOKEN", "env-token")
	t.Setenv("JXH_WPS_SID", "env-sid")
	t.Setenv("JXH_MYSQL_PASSWORD", "env-password")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OneBot.AccessToken != "env-token" {
		t.Fatalf("access token = %q", cfg.OneBot.AccessToken)
	}
	if cfg.WPSExcel.SID != "env-sid" {
		t.Fatalf("wps sid = %q", cfg.WPSExcel.SID)
	}
	if cfg.Database.Password != "env-password" {
		t.Fatalf("mysql password = %q", cfg.Database.Password)
	}
	if cfg.OneBot.APITimeout() != 30*time.Second {
		t.Fatalf("api timeout = %s", cfg.OneBot.APITimeout())
	}
}

func TestDatabaseDSNUsesExplicitEnv(t *testing.T) {
	cfg := Config{}
	t.Setenv("JXH_MYSQL_DSN", "user:pass@tcp(db:3306)/name?parseTime=True")
	if got := cfg.Database.DSN(); got != "user:pass@tcp(db:3306)/name?parseTime=True" {
		t.Fatalf("DSN() = %q", got)
	}
}

func TestDatabaseDSNBuildsFromFields(t *testing.T) {
	cfg := Config{Database: DatabaseConfig{
		Host: "mysql", Port: 3306, User: "jxh", Password: "secret",
		Name: "jxh_bot", Charset: "utf8mb4", ParseTime: true, Loc: "Local",
	}}
	want := "jxh:secret@tcp(mysql:3306)/jxh_bot?charset=utf8mb4&parseTime=True&loc=Local"
	if got := cfg.Database.DSN(); got != want {
		t.Fatalf("DSN() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run the config tests and verify they fail**

Run:

```bash
go test ./internal/config
```

Expected: FAIL because `go.mod`, `internal/config`, and `Load` do not exist.

- [ ] **Step 3: Create module and config implementation**

Create `go.mod`:

```go
module github.com/zjutjh/jxh-go

go 1.25
```

Create `.gitignore`:

```gitignore
.DS_Store
bin/
data/
config/config.yaml
*.log
```

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App       AppConfig       `yaml:"app"`
	Server    ServerConfig    `yaml:"server"`
	OneBot    OneBotConfig    `yaml:"onebot"`
	WPSExcel  WPSExcelConfig  `yaml:"wps_excel"`
	Database  DatabaseConfig  `yaml:"database"`
	Quote     QuoteConfig     `yaml:"quote"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Debug     DebugConfig     `yaml:"debug"`
}

type AppConfig struct {
	Debug    bool   `yaml:"debug"`
	LogLevel string `yaml:"log_level"`
	Timezone string `yaml:"timezone"`
}

type ServerConfig struct {
	Addr       string `yaml:"addr"`
	OneBotPath string `yaml:"onebot_path"`
}

type OneBotConfig struct {
	AccessToken  string `yaml:"access_token"`
	APITimeoutSec int   `yaml:"api_timeout_sec"`
}

func (c OneBotConfig) APITimeout() time.Duration {
	if c.APITimeoutSec <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.APITimeoutSec) * time.Second
}

type WPSExcelConfig struct {
	ShareURL  string `yaml:"share_url"`
	SID       string `yaml:"sid"`
	Sheet     string `yaml:"sheet"`
	CacheFile string `yaml:"cache_file"`
}

type DatabaseConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	User      string `yaml:"user"`
	Password  string `yaml:"password"`
	Name      string `yaml:"name"`
	Charset   string `yaml:"charset"`
	ParseTime bool   `yaml:"parse_time"`
	Loc       string `yaml:"loc"`
}

func (c DatabaseConfig) DSN() string {
	if dsn := os.Getenv("JXH_MYSQL_DSN"); dsn != "" {
		return dsn
	}
	parseTime := "False"
	if c.ParseTime {
		parseTime = "True"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%s&loc=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.Charset, parseTime, c.Loc)
}

type QuoteConfig struct {
	BaseURL    string `yaml:"base_url"`
	TimeoutSec int   `yaml:"timeout_sec"`
}

func (c QuoteConfig) Timeout() time.Duration {
	if c.TimeoutSec <= 0 {
		return 10 * time.Second
	}
	return time.Duration(c.TimeoutSec) * time.Second
}

type SchedulerConfig struct {
	Timezone string `yaml:"timezone"`
}

type DebugConfig struct {
	EnableTestCommand bool `yaml:"enable_test_command"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if token := os.Getenv("JXH_ONEBOT_TOKEN"); token != "" {
		cfg.OneBot.AccessToken = token
	}
	if sid := os.Getenv("JXH_WPS_SID"); sid != "" {
		cfg.WPSExcel.SID = sid
	}
	if password := os.Getenv("JXH_MYSQL_PASSWORD"); password != "" {
		cfg.Database.Password = password
	}
	return &cfg, nil
}
```

Create `config/config.example.yaml`:

```yaml
app:
  debug: false
  log_level: "info"
  timezone: "Asia/Shanghai"

server:
  addr: ":8080"
  onebot_path: "/onebot/v11/ws"

onebot:
  access_token: ""
  api_timeout_sec: 30

wps_excel:
  share_url: ""
  sid: ""
  sheet: "release"
  cache_file: "./data/cache/replies.xlsx"

database:
  host: "mysql"
  port: 3306
  user: "jxh"
  password: ""
  name: "jxh_bot"
  charset: "utf8mb4"
  parse_time: true
  loc: "Local"

quote:
  base_url: "http://quote:5000"
  timeout_sec: 10

scheduler:
  timezone: "Asia/Shanghai"

debug:
  enable_test_command: true
```

Create `internal/logger/logger.go`:

```go
package logger

import "go.uber.org/zap"

func New(level string, debug bool) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	if debug {
		cfg = zap.NewDevelopmentConfig()
	}
	if level != "" {
		if err := cfg.Level.UnmarshalText([]byte(level)); err != nil {
			return nil, err
		}
	}
	return cfg.Build()
}
```

- [ ] **Step 4: Run tests and tidy**

Run:

```bash
go get github.com/jellydator/ttlcache/v3@v3.4.0
go get github.com/robfig/cron/v3@v3.0.1
go get github.com/xuri/excelize/v2@v2.9.1
go get github.com/zjutjh/napcat-sdk@master
go get go.uber.org/zap@v1.27.0
go get gopkg.in/yaml.v3@v3.0.1
go get gorm.io/driver/mysql@v1.5.7
go get gorm.io/gorm@v1.25.12
go mod tidy
go test ./internal/config
```

Expected: PASS for `./internal/config`.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum .gitignore config/config.example.yaml internal/config internal/logger
git commit -m "feat: initialize bot config and logging"
```

---

### Task 2: Define Domain Types And NapCat Adapter Boundary

**Files:**
- Create: `internal/domain/message.go`
- Create: `internal/domain/result.go`
- Create: `internal/napcat/adapter.go`
- Create: `internal/napcat/actions.go`
- Create: `internal/napcat/adapter_test.go`

- [ ] **Step 1: Write failing adapter tests**

Create `internal/napcat/adapter_test.go`:

```go
package napcat

import (
	"testing"

	"github.com/zjutjh/jxh-go/internal/domain"
	sdkevent "github.com/zjutjh/napcat-sdk/event"
	"github.com/zjutjh/napcat-sdk/message"
)

func TestAdaptGroupMessageExtractsTextAtReplyAndImage(t *testing.T) {
	ev := &sdkevent.GroupMessage{
		GroupID: 100,
		UserID:  200,
		MessageID: 300,
		Message: message.ChainOf(
			message.Reply(123),
			message.At(456),
			message.Text(" /q "),
			message.Image("base64://abc"),
		),
	}
	msg, ok := AdaptGroupMessage(ev)
	if !ok {
		t.Fatal("AdaptGroupMessage returned false")
	}
	if msg.GroupID != 100 || msg.UserID != 200 || msg.MessageID != 300 {
		t.Fatalf("ids = %#v", msg)
	}
	if msg.Text != "/q" {
		t.Fatalf("Text = %q", msg.Text)
	}
	if len(msg.AtUserIDs) != 1 || msg.AtUserIDs[0] != 456 {
		t.Fatalf("AtUserIDs = %#v", msg.AtUserIDs)
	}
	if msg.ReplyTo == nil || *msg.ReplyTo != 123 {
		t.Fatalf("ReplyTo = %#v", msg.ReplyTo)
	}
	if len(msg.Images) != 1 || msg.Images[0] != "base64://abc" {
		t.Fatalf("Images = %#v", msg.Images)
	}
}

func TestMessageFromResultBuildsSDKChain(t *testing.T) {
	result := domain.Message{Segments: []domain.Segment{
		{Type: domain.SegmentReply, Value: "12"},
		{Type: domain.SegmentAt, Value: "34"},
		{Type: domain.SegmentText, Value: "hello"},
	}}
	chain := ChainFromMessage(result)
	if got := chain.Text(); got != "hello" {
		t.Fatalf("Text() = %q", got)
	}
	if len(chain.OfType("reply")) != 1 {
		t.Fatalf("reply count = %d", len(chain.OfType("reply")))
	}
	if len(chain.OfType("at")) != 1 {
		t.Fatalf("at count = %d", len(chain.OfType("at")))
	}
}
```

- [ ] **Step 2: Run adapter tests and verify they fail**

Run:

```bash
go test ./internal/napcat
```

Expected: FAIL because domain and adapter types do not exist.

- [ ] **Step 3: Implement domain types**

Create `internal/domain/message.go`:

```go
package domain

type GroupMessage struct {
	GroupID   int64
	UserID    int64
	MessageID int64
	SelfID    int64
	Text      string
	AtUserIDs []int64
	ReplyTo   *int64
	Images    []string
	IsSelf    bool
}

type SegmentType string

const (
	SegmentText  SegmentType = "text"
	SegmentAt    SegmentType = "at"
	SegmentReply SegmentType = "reply"
	SegmentImage SegmentType = "image"
)

type Segment struct {
	Type  SegmentType
	Value string
}

type Message struct {
	Segments []Segment
}

func TextMessage(text string) Message {
	return Message{Segments: []Segment{{Type: SegmentText, Value: text}}}
}
```

Create `internal/domain/result.go`:

```go
package domain

type CommandResult struct {
	GroupID int64
	Message Message
	Handled bool
}
```

- [ ] **Step 4: Implement NapCat adapter helpers**

Create `internal/napcat/adapter.go`:

```go
package napcat

import (
	"strconv"
	"strings"

	"github.com/zjutjh/jxh-go/internal/domain"
	sdkevent "github.com/zjutjh/napcat-sdk/event"
	"github.com/zjutjh/napcat-sdk/message"
)

func AdaptGroupMessage(ev *sdkevent.GroupMessage) (domain.GroupMessage, bool) {
	if ev == nil {
		return domain.GroupMessage{}, false
	}
	var atIDs []int64
	var images []string
	var replyTo *int64
	for _, seg := range ev.Message {
		switch seg.Type {
		case "at":
			if id, err := strconv.ParseInt(seg.String("qq"), 10, 64); err == nil {
				atIDs = append(atIDs, id)
			}
		case "reply":
			if id, err := strconv.ParseInt(seg.String("id"), 10, 64); err == nil {
				replyTo = &id
			}
		case "image":
			images = append(images, seg.String("file"))
		}
	}
	return domain.GroupMessage{
		GroupID: ev.GroupID, UserID: ev.UserID, MessageID: ev.MessageID,
		SelfID: ev.SelfID, Text: strings.TrimSpace(ev.Message.Text()),
		AtUserIDs: atIDs, ReplyTo: replyTo, Images: images,
		IsSelf: ev.UserID == ev.SelfID,
	}, true
}

func ChainFromMessage(msg domain.Message) message.Chain {
	segments := make([]message.Segment, 0, len(msg.Segments))
	for _, seg := range msg.Segments {
		switch seg.Type {
		case domain.SegmentText:
			segments = append(segments, message.Text(seg.Value))
		case domain.SegmentAt:
			if id, err := strconv.ParseInt(seg.Value, 10, 64); err == nil {
				segments = append(segments, message.At(id))
			}
		case domain.SegmentReply:
			if id, err := strconv.ParseInt(seg.Value, 10, 64); err == nil {
				segments = append(segments, message.Reply(id))
			}
		case domain.SegmentImage:
			segments = append(segments, message.Image(seg.Value))
		}
	}
	return message.ChainOf(segments...)
}
```

Create `internal/napcat/actions.go`:

```go
package napcat

import (
	"context"
	"strconv"

	"github.com/zjutjh/jxh-go/internal/domain"
	"github.com/zjutjh/napcat-sdk"
	"github.com/zjutjh/napcat-sdk/api"
)

type ActionClient interface {
	SendGroupMessage(ctx context.Context, groupID int64, msg domain.Message) (int64, error)
	GetMessage(ctx context.Context, messageID int64) (map[string]any, error)
	SetGroupBan(ctx context.Context, groupID, userID int64, durationSec int) error
	Restart(ctx context.Context, delayMS int) error
}

type SDKActions struct {
	client *napcat.Client
}

func NewSDKActions(client *napcat.Client) *SDKActions {
	return &SDKActions{client: client}
}

func (a *SDKActions) SendGroupMessage(ctx context.Context, groupID int64, msg domain.Message) (int64, error) {
	resp, err := a.client.API().SendGroupMsg(ctx, api.SendGroupMsgRequest{
		GroupID: strconv.FormatInt(groupID, 10),
		Message: ChainFromMessage(msg),
	})
	if err != nil {
		return 0, err
	}
	return resp.MessageID, nil
}

func (a *SDKActions) GetMessage(ctx context.Context, messageID int64) (map[string]any, error) {
	resp, err := a.client.API().GetMsg(ctx, api.GetMsgRequest{MessageID: strconv.FormatInt(messageID, 10)})
	if err != nil {
		return nil, err
	}
	return resp.Raw, nil
}

func (a *SDKActions) SetGroupBan(ctx context.Context, groupID, userID int64, durationSec int) error {
	_, err := a.client.API().SetGroupBan(ctx, api.SetGroupBanRequest{
		GroupID: strconv.FormatInt(groupID, 10),
		UserID: strconv.FormatInt(userID, 10),
		Duration: durationSec,
	})
	return err
}

func (a *SDKActions) Restart(ctx context.Context, delayMS int) error {
	_, err := a.client.API().SetRestart(ctx, api.SetRestartRequest{Delay: delayMS})
	return err
}
```

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/domain ./internal/napcat
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain internal/napcat go.mod go.sum
git commit -m "feat: add napcat sdk adapter boundary"
```

---

### Task 3: Add MySQL GORM Storage

**Files:**
- Create: `internal/storage/models.go`
- Create: `internal/storage/db.go`
- Create: `internal/storage/admins.go`
- Create: `internal/storage/blacklist.go`
- Create: `internal/storage/replies.go`
- Create: `internal/storage/jobs.go`
- Create: `internal/storage/storage_test.go`

- [ ] **Step 1: Write repository tests**

Create `internal/storage/storage_test.go`:

```go
package storage

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := getenv("JXH_TEST_MYSQL_DSN", "root:root@tcp(127.0.0.1:3306)/jxh_bot_test?charset=utf8mb4&parseTime=True&loc=Local")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("test MySQL unavailable: %v", err)
	}
	if err := db.AutoMigrate(&Admin{}, &BlacklistEntry{}, &ReplyRule{}, &ScheduledJob{}); err != nil {
		t.Fatal(err)
	}
	db.Exec("DELETE FROM admins")
	db.Exec("DELETE FROM blacklist")
	db.Exec("DELETE FROM reply_rules")
	db.Exec("DELETE FROM scheduled_jobs")
	return db
}

func TestAdminRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewAdminRepository(openTestDB(t))
	if err := repo.Add(ctx, 123); err != nil {
		t.Fatal(err)
	}
	admins, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(admins) != 1 || admins[0] != 123 {
		t.Fatalf("admins = %#v", admins)
	}
	if err := repo.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	admins, _ = repo.List(ctx)
	if len(admins) != 0 {
		t.Fatalf("admins after clear = %#v", admins)
	}
}

func TestReplyRuleReplaceAll(t *testing.T) {
	ctx := context.Background()
	repo := NewReplyRepository(openTestDB(t))
	rules := []ReplyRule{{Keyword: "菜单", Reply: "功能列表", UpdatedAt: time.Now()}}
	if err := repo.ReplaceAll(ctx, rules); err != nil {
		t.Fatal(err)
	}
	got, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Keyword != "菜单" || got[0].Reply != "功能列表" {
		t.Fatalf("rules = %#v", got)
	}
}
```

- [ ] **Step 2: Run storage tests and verify they fail**

Run:

```bash
go test ./internal/storage
```

Expected: FAIL because storage package does not exist.

- [ ] **Step 3: Implement models and repositories**

Create `internal/storage/models.go`:

```go
package storage

import "time"

type Admin struct {
	UserID    int64     `gorm:"primaryKey;column:user_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (Admin) TableName() string { return "admins" }

type BlacklistEntry struct {
	UserID    int64     `gorm:"primaryKey;column:user_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (BlacklistEntry) TableName() string { return "blacklist" }

type ReplyRule struct {
	Keyword   string    `gorm:"primaryKey;size:255;column:keyword"`
	Reply     string    `gorm:"type:text;column:reply;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (ReplyRule) TableName() string { return "reply_rules" }

type ScheduledJob struct {
	ID        int64      `gorm:"primaryKey;autoIncrement;column:id"`
	Type      string     `gorm:"size:16;column:type;not null"`
	TimeHHMM  string     `gorm:"size:5;column:time_hhmm;not null"`
	GroupID   int64      `gorm:"column:group_id;not null"`
	Message   string     `gorm:"type:text;column:message;not null"`
	Enabled   bool       `gorm:"column:enabled;not null"`
	LastRunAt *time.Time `gorm:"column:last_run_at"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (ScheduledJob) TableName() string { return "scheduled_jobs" }
```

Create `internal/storage/db.go`:

```go
package storage

import (
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, AutoMigrate(db)
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&Admin{}, &BlacklistEntry{}, &ReplyRule{}, &ScheduledJob{})
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
```

Create repository files with these exported constructors and methods:

```go
func NewAdminRepository(db *gorm.DB) *AdminRepository
func (r *AdminRepository) Add(ctx context.Context, userID int64) error
func (r *AdminRepository) Remove(ctx context.Context, userID int64) error
func (r *AdminRepository) Clear(ctx context.Context) error
func (r *AdminRepository) List(ctx context.Context) ([]int64, error)
func (r *AdminRepository) Exists(ctx context.Context, userID int64) (bool, error)
```

```go
func NewBlacklistRepository(db *gorm.DB) *BlacklistRepository
func (r *BlacklistRepository) Add(ctx context.Context, userID int64) error
func (r *BlacklistRepository) Remove(ctx context.Context, userID int64) error
func (r *BlacklistRepository) Clear(ctx context.Context) error
func (r *BlacklistRepository) List(ctx context.Context) ([]int64, error)
func (r *BlacklistRepository) Exists(ctx context.Context, userID int64) (bool, error)
```

```go
func NewReplyRepository(db *gorm.DB) *ReplyRepository
func (r *ReplyRepository) List(ctx context.Context) ([]ReplyRule, error)
func (r *ReplyRepository) ReplaceAll(ctx context.Context, rules []ReplyRule) error
```

```go
func NewJobRepository(db *gorm.DB) *JobRepository
func (r *JobRepository) Create(ctx context.Context, job ScheduledJob) (ScheduledJob, error)
func (r *JobRepository) ListEnabled(ctx context.Context) ([]ScheduledJob, error)
func (r *JobRepository) Delete(ctx context.Context, id int64) error
func (r *JobRepository) Disable(ctx context.Context, id int64) error
```

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/storage
```

Expected: PASS when test MySQL is available; SKIP with `test MySQL unavailable` when no local MySQL is running.

- [ ] **Step 5: Commit**

```bash
git add internal/storage go.mod go.sum
git commit -m "feat: add mysql gorm repositories"
```

---

### Task 4: Implement Reply Rule Loading From WPS Excel

**Files:**
- Create: `internal/reply/service.go`
- Create: `internal/reply/wps.go`
- Create: `internal/reply/wps_test.go`

- [ ] **Step 1: Write failing reply tests**

Create `internal/reply/wps_test.go`:

```go
package reply

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseReleaseSheet(t *testing.T) {
	f := excelize.NewFile()
	sheet := "release"
	f.NewSheet(sheet)
	f.SetCellValue(sheet, "A1", "菜单")
	f.SetCellValue(sheet, "B1", "功能列表")
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	rules, err := ParseExcel(bytes.NewReader(buf.Bytes()), sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Keyword != "菜单" || rules[0].Reply != "功能列表" {
		t.Fatalf("rules = %#v", rules)
	}
}

func TestWPSClientDownloadsExcel(t *testing.T) {
	excelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := excelize.NewFile()
		f.NewSheet("release")
		f.SetCellValue("release", "A1", "bot")
		f.SetCellValue("release", "B1", "在线")
		_ = f.Write(w)
	}))
	defer excelSrv.Close()
	shareSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "wps_sid=sid-value" {
			t.Fatalf("cookie = %q", got)
		}
		w.Write([]byte(`{"download_url":"` + excelSrv.URL + `"}`))
	}))
	defer shareSrv.Close()

	client := NewWPSClient(http.DefaultClient)
	data, err := client.Download(context.Background(), shareSrv.URL, "sid-value")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("downloaded data is empty")
	}
}
```

- [ ] **Step 2: Run reply tests and verify they fail**

Run:

```bash
go test ./internal/reply
```

Expected: FAIL because reply package does not exist.

- [ ] **Step 3: Implement reply parsing and service**

Create `internal/reply/wps.go` with:

```go
func ParseExcel(r io.Reader, sheet string) ([]storage.ReplyRule, error)
func NewWPSClient(client *http.Client) *WPSClient
func (c *WPSClient) Download(ctx context.Context, shareURL, sid string) ([]byte, error)
```

Implementation requirements:

- `ParseExcel` reads each row from `sheet`.
- Trim keyword and reply.
- Skip rows with empty keyword.
- Return an error for duplicate keyword.
- Set `UpdatedAt` to `time.Now()` for each rule.
- `Download` sends `wps_sid=<sid>` cookie to the share URL.
- `Download` expects JSON field `download_url`.
- `Download` fetches the binary Excel file from `download_url`.

Create `internal/reply/service.go` with:

```go
type Store interface {
	List(ctx context.Context) ([]storage.ReplyRule, error)
	ReplaceAll(ctx context.Context, rules []storage.ReplyRule) error
}

type Service struct {
	store Store
	rules map[string]string
}

func NewService(store Store) *Service
func (s *Service) Load(ctx context.Context) error
func (s *Service) Replace(ctx context.Context, rules []storage.ReplyRule) error
func (s *Service) Match(text string) (string, bool)
```

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/reply
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/reply go.mod go.sum
git commit -m "feat: load reply rules from wps excel"
```

---

### Task 5: Implement Command Router And Admin Commands

**Files:**
- Create: `internal/commands/router.go`
- Create: `internal/commands/admin.go`
- Create: `internal/commands/test.go`
- Create: `internal/commands/router_test.go`
- Create: `internal/commands/admin_test.go`

- [ ] **Step 1: Write failing command tests**

Create `internal/commands/router_test.go`:

```go
package commands

import (
	"context"
	"testing"

	"github.com/zjutjh/jxh-go/internal/domain"
)

func TestRouterMatchesAdminCommand(t *testing.T) {
	router := NewRouter()
	router.Register(CommandFunc{
		NameValue: "admin",
		MatchFunc: func(msg domain.GroupMessage) bool { return msg.Text == "/admin 所有管理员" },
		ExecFunc: func(context.Context, Env, domain.GroupMessage) (domain.CommandResult, error) {
			return domain.CommandResult{Handled: true, GroupID: 1, Message: domain.TextMessage("admins")}, nil
		},
	})
	result, handled, err := router.Dispatch(context.Background(), Env{}, domain.GroupMessage{GroupID: 1, Text: "/admin 所有管理员"})
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !result.Handled || result.Message.Segments[0].Value != "admins" {
		t.Fatalf("result = %#v handled=%v", result, handled)
	}
}
```

Create `internal/commands/admin_test.go`:

```go
package commands

import (
	"context"
	"testing"

	"github.com/zjutjh/jxh-go/internal/domain"
)

func TestAdminCommandRejectsNonAdmin(t *testing.T) {
	env := NewFakeEnv()
	cmd := NewAdminCommand()
	result, err := cmd.Execute(context.Background(), env, domain.GroupMessage{
		GroupID: 1, UserID: 100, Text: "/admin 所有管理员",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Message.Segments[0].Value != "你没有权限执行此操作" {
		t.Fatalf("message = %#v", result.Message)
	}
}

func TestAdminCommandAddsAdminFromAt(t *testing.T) {
	env := NewFakeEnv()
	env.SelfID = 100
	cmd := NewAdminCommand()
	result, err := cmd.Execute(context.Background(), env, domain.GroupMessage{
		GroupID: 1, UserID: 100, SelfID: 100, IsSelf: true,
		Text: "/admin 添加管理员", AtUserIDs: []int64{200},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !env.Admins[200] {
		t.Fatal("admin 200 was not added")
	}
	if result.Message.Segments[0].Value != "已添加管理员: 200" {
		t.Fatalf("message = %#v", result.Message)
	}
}
```

- [ ] **Step 2: Run command tests and verify they fail**

Run:

```bash
go test ./internal/commands
```

Expected: FAIL because command router does not exist.

- [ ] **Step 3: Implement command router and admin command**

Create `internal/commands/router.go` with:

```go
type Env struct {
	Actions napcat.ActionClient
	Admins AdminStore
	Blacklist BlacklistStore
	Replies ReplyReloader
}

type Command interface {
	Name() string
	Match(domain.GroupMessage) bool
	Execute(context.Context, Env, domain.GroupMessage) (domain.CommandResult, error)
}

type Router struct { commands []Command }
func NewRouter() *Router
func (r *Router) Register(cmd Command)
func (r *Router) Dispatch(ctx context.Context, env Env, msg domain.GroupMessage) (domain.CommandResult, bool, error)
```

Create `internal/commands/admin.go` supporting exact old command strings:

- `/admin 添加管理员`
- `/admin 移除管理员`
- `/admin 移除所有管理员`
- `/admin 所有管理员`
- `/admin 添加黑名单`
- `/admin 移除黑名单`
- `/admin 移除所有黑名单`
- `/admin 所有黑名单`
- `/admin ban <duration>`
- `/admin restart`
- `/admin 定时任务 查看`
- `/admin 定时任务 添加 <每天|单次> <时间> <群聊ID> <消息内容>`
- `/admin 定时任务 移除 <任务编号>`

Create `internal/commands/test.go` with `/test` response containing a compact JSON-like debug string from the incoming message.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/commands
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/commands internal/domain
git commit -m "feat: add command router and admin commands"
```

---

### Task 6: Implement Quote Client And `/q`

**Files:**
- Create: `internal/quote/client.go`
- Create: `internal/quote/client_test.go`
- Create: `internal/commands/quote.go`
- Modify: `internal/commands/router.go`
- Test: `internal/commands/quote_test.go`

- [ ] **Step 1: Write failing quote tests**

Create `internal/quote/client_test.go`:

```go
package quote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateBase64(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/base64/" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var body []Request
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 1 || body[0].Message != "hello" {
			t.Fatalf("body = %#v", body)
		}
		w.Write([]byte("abc"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, http.DefaultClient)
	got, err := client.GenerateBase64(context.Background(), Request{Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "base64://abc" {
		t.Fatalf("got = %q", got)
	}
}
```

- [ ] **Step 2: Run quote tests and verify they fail**

Run:

```bash
go test ./internal/quote ./internal/commands
```

Expected: FAIL because quote client and `/q` command are missing.

- [ ] **Step 3: Implement quote client**

Create `internal/quote/client.go`:

```go
type Request struct {
	UserID int64 `json:"user_id,omitempty"`
	UserNickname string `json:"user_nickname,omitempty"`
	Message string `json:"message,omitempty"`
	Image []string `json:"image,omitempty"`
	Reply *Request `json:"reply,omitempty"`
}

type Client struct { baseURL string; httpClient *http.Client }
func NewClient(baseURL string, httpClient *http.Client) *Client
func (c *Client) GenerateBase64(ctx context.Context, req Request) (string, error)
```

Implementation sends JSON array `[req]` to `<baseURL>/base64/` and returns `"base64://"+responseText`.

- [ ] **Step 4: Implement `/q` command**

Create `internal/commands/quote.go`:

```go
type QuoteClient interface {
	GenerateBase64(ctx context.Context, req quote.Request) (string, error)
}

func NewQuoteCommand(client QuoteClient) Command
```

Behavior:

- Match only `msg.Text == "/q"`.
- If `msg.ReplyTo == nil`, return `请回复一条消息后再使用 /q`。
- Call `Actions.GetMessage(ctx, *msg.ReplyTo)`.
- Build `quote.Request` from returned message text, images, user id, and nickname.
- Call quote client and send returned image segment through command result.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/quote ./internal/commands
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/quote internal/commands
git commit -m "feat: add quote command"
```

---

### Task 7: Implement Bot Pipeline And Keyword Fallback

**Files:**
- Create: `internal/bot/pipeline.go`
- Create: `internal/bot/permissions.go`
- Create: `internal/bot/pipeline_test.go`
- Modify: `internal/reply/service.go`

- [ ] **Step 1: Write failing pipeline tests**

Create `internal/bot/pipeline_test.go`:

```go
package bot

import (
	"context"
	"testing"

	"github.com/zjutjh/jxh-go/internal/domain"
)

func TestPipelineIgnoresBlacklistedUser(t *testing.T) {
	env := NewFakeEnv()
	env.Blacklist[100] = true
	p := NewPipeline(env)
	result, handled, err := p.HandleGroupMessage(context.Background(), domain.GroupMessage{
		GroupID: 1, UserID: 100, Text: "菜单",
	})
	if err != nil {
		t.Fatal(err)
	}
	if handled || result.Handled {
		t.Fatalf("blacklisted message handled: %#v", result)
	}
}

func TestPipelineUsesKeywordFallback(t *testing.T) {
	env := NewFakeEnv()
	env.Replies["菜单"] = "功能列表"
	p := NewPipeline(env)
	result, handled, err := p.HandleGroupMessage(context.Background(), domain.GroupMessage{
		GroupID: 1, UserID: 100, Text: "菜单",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !handled || result.Message.Segments[0].Value != "功能列表" {
		t.Fatalf("result = %#v handled=%v", result, handled)
	}
}
```

- [ ] **Step 2: Run pipeline tests and verify they fail**

Run:

```bash
go test ./internal/bot
```

Expected: FAIL because bot pipeline does not exist.

- [ ] **Step 3: Implement pipeline**

Create `internal/bot/pipeline.go`:

```go
type Env struct {
	Router *commands.Router
	CommandEnv commands.Env
	Blacklist interface{ Exists(context.Context, int64) (bool, error) }
	Replies interface{ Match(string) (string, bool) }
}

type Pipeline struct { env Env }
func NewPipeline(env Env) *Pipeline
func (p *Pipeline) HandleGroupMessage(ctx context.Context, msg domain.GroupMessage) (domain.CommandResult, bool, error)
```

Processing order:

1. If user is blacklisted and not self, return not handled.
2. Dispatch commands.
3. If no command handled and `!msg.IsSelf`, exact-match keyword reply.
4. Return not handled.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/bot
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bot internal/reply
git commit -m "feat: add group message pipeline"
```

---

### Task 8: Implement Scheduler

**Files:**
- Create: `internal/scheduler/scheduler.go`
- Create: `internal/scheduler/scheduler_test.go`
- Modify: `internal/commands/admin.go`

- [ ] **Step 1: Write failing scheduler tests**

Create `internal/scheduler/scheduler_test.go`:

```go
package scheduler

import "testing"

func TestNextRunDailyFutureToday(t *testing.T) {
	now := mustTime("2026-06-16 10:00")
	next, err := NextRun("每天", "11:30", now)
	if err != nil {
		t.Fatal(err)
	}
	if next.Format("2006-01-02 15:04") != "2026-06-16 11:30" {
		t.Fatalf("next = %s", next.Format("2006-01-02 15:04"))
	}
}

func TestNextRunOncePastGoesTomorrow(t *testing.T) {
	now := mustTime("2026-06-16 12:00")
	next, err := NextRun("单次", "11:30", now)
	if err != nil {
		t.Fatal(err)
	}
	if next.Format("2006-01-02 15:04") != "2026-06-17 11:30" {
		t.Fatalf("next = %s", next.Format("2006-01-02 15:04"))
	}
}
```

- [ ] **Step 2: Run scheduler tests and verify they fail**

Run:

```bash
go test ./internal/scheduler
```

Expected: FAIL because scheduler package does not exist.

- [ ] **Step 3: Implement scheduler**

Create `internal/scheduler/scheduler.go`:

```go
func NextRun(jobType, hhmm string, now time.Time) (time.Time, error)
type ActionClient interface {
	SendGroupMessage(ctx context.Context, groupID int64, msg domain.Message) (int64, error)
}
type JobStore interface {
	ListEnabled(ctx context.Context) ([]storage.ScheduledJob, error)
	Create(ctx context.Context, job storage.ScheduledJob) (storage.ScheduledJob, error)
	Delete(ctx context.Context, id int64) error
	Disable(ctx context.Context, id int64) error
}
type Scheduler struct
func New(store JobStore, actions ActionClient, loc *time.Location) *Scheduler
func (s *Scheduler) Start(ctx context.Context) error
func (s *Scheduler) Add(ctx context.Context, job storage.ScheduledJob) (storage.ScheduledJob, error)
func (s *Scheduler) Remove(ctx context.Context, id int64) error
```

Requirements:

- Use `robfig/cron/v3`.
- Daily jobs remain enabled after send.
- Once jobs disable after successful send.
- No distributed lock.

- [ ] **Step 4: Wire admin scheduled-job subcommands**

Modify `internal/commands/admin.go` so:

- `/admin 定时任务 查看` lists enabled jobs with index/id, type, time, group id, message.
- `/admin 定时任务 添加 <每天|单次> <时间> <群聊ID> <消息内容>` calls scheduler `Add`.
- `/admin 定时任务 移除 <任务编号>` calls scheduler `Remove`.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/scheduler ./internal/commands
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scheduler internal/commands
git commit -m "feat: add persisted scheduler"
```

---

### Task 9: Wire Application Entrypoint

**Files:**
- Create: `cmd/bot/main.go`
- Modify: `internal/napcat/adapter.go`
- Modify: `internal/bot/pipeline.go`

- [ ] **Step 1: Write a compile target**

Run:

```bash
go test ./...
```

Expected: FAIL because no entrypoint wires packages together yet, and some constructors required by `main` are missing.

- [ ] **Step 2: Implement main wiring**

Create `cmd/bot/main.go`:

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/zjutjh/jxh-go/internal/config"
	"github.com/zjutjh/jxh-go/internal/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	zlog, err := logger.New(cfg.App.LogLevel, cfg.App.Debug)
	if err != nil {
		log.Fatal(err)
	}
	defer zlog.Sync()

	httpClient := &http.Client{Timeout: cfg.Quote.Timeout()}
	_ = httpClient
	<-ctx.Done()
}
```

Then expand the entrypoint in the same task to:

1. Open MySQL with `storage.Open(cfg.Database.DSN())`.
2. Construct repositories.
3. Load reply rules.
4. Construct quote client.
5. Construct command router.
6. Construct bot pipeline.
7. Start scheduler.
8. Start `napcat.ServeReverseWebSocket` with `cfg.Server.Addr`.
9. On each SDK event, adapt group messages and pass them to pipeline.
10. If pipeline returns a handled result, call action client `SendGroupMessage`.

- [ ] **Step 3: Run all tests**

Run:

```bash
go test ./...
```

Expected: PASS, except storage integration tests may SKIP when test MySQL is unavailable.

- [ ] **Step 4: Commit**

```bash
git add cmd internal
git commit -m "feat: wire bot application"
```

---

### Task 10: Add Docker, Compose, And Runbook

**Files:**
- Create: `Dockerfile`
- Create: `compose.yaml`
- Create: `README.md`

- [ ] **Step 1: Add Dockerfile**

Create `Dockerfile`:

```dockerfile
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/jxh-bot ./cmd/bot

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/jxh-bot /app/jxh-bot
CMD ["/app/jxh-bot"]
```

- [ ] **Step 2: Add compose file**

Create `compose.yaml`:

```yaml
services:
  mysql:
    image: mysql:8.4
    restart: unless-stopped
    environment:
      MYSQL_DATABASE: jxh_bot
      MYSQL_USER: jxh
      MYSQL_PASSWORD: ${JXH_MYSQL_PASSWORD}
      MYSQL_ROOT_PASSWORD: ${JXH_MYSQL_ROOT_PASSWORD}
      TZ: Asia/Shanghai
    volumes:
      - ./data/mysql:/var/lib/mysql
    command:
      - --character-set-server=utf8mb4
      - --collation-server=utf8mb4_unicode_ci

  bot:
    build: .
    restart: unless-stopped
    depends_on:
      - mysql
      - quote
    volumes:
      - ./config/config.yaml:/app/config/config.yaml:ro
      - ./data/cache:/app/data/cache
    environment:
      JXH_ONEBOT_TOKEN: ${JXH_ONEBOT_TOKEN}
      JXH_WPS_SID: ${JXH_WPS_SID}
      JXH_MYSQL_PASSWORD: ${JXH_MYSQL_PASSWORD}
    ports:
      - "8080:8080"

  napcat:
    image: napcat/napcat:latest
    restart: unless-stopped
    depends_on:
      - bot
    volumes:
      - ./napcat:/app/.config/QQ
    ports:
      - "6099:6099"

  quote:
    image: zhullyb/qq-quote-generator
    restart: unless-stopped
    ports:
      - "5004:5000"
```

- [ ] **Step 3: Add README**

Create `README.md`:

````markdown
# 精小弘 Go Bot

单实例 Go + NapCat 重构版，只保留旧 `qqbot-JXH` 已实现功能。

## 本地配置

```bash
cp config/config.example.yaml config/config.yaml
export JXH_ONEBOT_TOKEN=change-me
export JXH_WPS_SID=change-me
export JXH_MYSQL_PASSWORD=change-me
export JXH_MYSQL_ROOT_PASSWORD=change-root
```

## 启动

```bash
docker compose up --build
```

NapCat WebUI 中配置 WebSocket Client：

```text
ws://bot:8080/onebot/v11/ws
```

Token 与 `JXH_ONEBOT_TOKEN` 保持一致。

## 不包含的功能

- `/ai`
- 管理后台
- MCP
- 长期记忆
- 主动聊天
````

- [ ] **Step 4: Verify Docker build**

Run:

```bash
docker compose config
docker build .
```

Expected: `docker compose config` exits 0 and `docker build .` exits 0.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile compose.yaml README.md
git commit -m "chore: add container deployment"
```

---

### Task 11: Final Verification

**Files:**
- Modify only files required by failures found during verification.

- [ ] **Step 1: Run full unit test suite**

Run:

```bash
go test ./...
```

Expected: PASS; storage tests may SKIP only when test MySQL is unavailable.

- [ ] **Step 2: Run formatting**

Run:

```bash
gofmt -w cmd internal
go test ./...
```

Expected: PASS; no formatting diffs after second `gofmt`.

- [ ] **Step 3: Run compose validation**

Run:

```bash
docker compose config
```

Expected: exit 0.

- [ ] **Step 4: Confirm no forbidden feature was added**

Run:

```bash
rg -n "/ai|MCP|Milvus|vector|admin web|dashboard|主动聊天|长期记忆" cmd internal README.md
```

Expected: no matches except README lines under "不包含的功能".

- [ ] **Step 5: Commit final fixes**

```bash
git status --short
git add .
git commit -m "test: verify go napcat bot migration"
```

Expected: Commit only if verification fixes changed tracked files. Do not add `docs/superpowers/.DS_Store`.

---

## Self-Review

Spec coverage:

- NapCat SDK reverse WebSocket: Task 2 and Task 9.
- MySQL + GORM single-instance persistence: Task 3.
- WPS Excel reply table and `/reload`: Task 4 and Task 5.
- Existing `/admin` commands: Task 5 and Task 8.
- `/q` quote service: Task 6.
- Blacklist and keyword fallback: Task 7.
- Scheduler independent from WebSocket receive loop: Task 8.
- Docker Compose with NapCat, bot, quote, mysql: Task 10.
- No `/ai` or new features: Task 11 verification.

No planned task requires multi-instance coordination, distributed locks, MCP, AI, long-term memory, admin UI, or vector storage.
