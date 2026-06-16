# 精小弘 Go + NapCat + Eino 重构 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 从空仓库落地一个单实例 Go bot，保留 qqbot-JXH 的关键词回复、WPS reload、管理员/黑名单、引用图、定时任务，并新增基于同一知识库的 `/ai` RAG 问答。

**Architecture:** 采用分层包结构：`cmd/bot` 负责启动，`internal/napcat` 适配 OneBot/NapCat，`internal/bot` 管消息管线，`internal/knowledge` 管 WPS 解析与检索，`internal/storage` 管 GORM/MySQL，`internal/ai` 管 RAG，`internal/cache` 管可重建派生缓存。WPS 仍以两列为主，导入器自动解析 `%编号` 菜单树并生成 RAG `content`。

**Tech Stack:** Go 1.25、`github.com/zjutjh/napcat-sdk`、GORM + MySQL、Eino/OpenAI-compatible ChatModel 与 Embedder、excelize、zap、yaml.v3、robfig/cron。

---

## File Structure

- Create `go.mod`, `go.sum`: Go module and dependencies.
- Create `cmd/bot/main.go`: load config, logger, storage, knowledge cache, scheduler, NapCat reverse WS server.
- Create `internal/config/config.go`: YAML config and env overrides.
- Create `internal/storage/models.go`, `internal/storage/store.go`: GORM models, migrations, repositories.
- Create `internal/knowledge/types.go`, `parser.go`, `index.go`, `retriever.go`, `sync.go`: WPS parsing, menu tree enrichment, keyword index, hybrid text retrieval, import orchestration.
- Create `internal/cache/cache.go`: atomic keyword index cache and short TTL event cache.
- Create `internal/ai/service.go`: RAG prompt assembly and ChatModel abstraction with OpenAI-compatible implementation hook.
- Create `internal/vector/vector.go`: embedding/vector store interfaces plus in-memory/noop implementation for test and disabled vector mode.
- Create `internal/commands/admin.go`, `commands.go`: command parsing and admin command handling.
- Create `internal/bot/pipeline.go`: group message processing.
- Create `internal/napcat/adapter.go`: SDK event/API adapter.
- Create `internal/quote/client.go`: quote-generator HTTP client.
- Create `internal/scheduler/scheduler.go`: scheduled job runtime.
- Create `config.example.yaml`, `Dockerfile`, `compose.yaml`, `README.md`: deploy and run docs.

## Task 1: Project Skeleton and Config

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `config.example.yaml`

- [ ] **Step 1: Write failing config tests**

```go
func TestLoadAppliesEnvOverrides(t *testing.T) {
    t.Setenv("JXH_ONEBOT_TOKEN", "token-from-env")
    t.Setenv("JXH_MYSQL_DSN", "user:pass@tcp(localhost:3306)/jxh?charset=utf8mb4&parseTime=True&loc=Local")
    cfg, err := config.Load("testdata/minimal.yaml")
    require.NoError(t, err)
    require.Equal(t, "token-from-env", cfg.OneBot.AccessToken)
    require.Equal(t, "user:pass@tcp(localhost:3306)/jxh?charset=utf8mb4&parseTime=True&loc=Local", cfg.Database.DSN)
}
```

- [ ] **Step 2: Run red test**

Run: `go test ./internal/config`

Expected: package missing or test fails because `Load` does not exist.

- [ ] **Step 3: Implement config loader**

Implement typed config structs, defaults, YAML parsing, and env overrides for `JXH_ONEBOT_TOKEN`, `JXH_WPS_SID`, `JXH_MYSQL_PASSWORD`, `JXH_MYSQL_DSN`, `JXH_AI_BASE_URL`, `JXH_AI_API_KEY`, `JXH_EMBEDDING_BASE_URL`, `JXH_EMBEDDING_API_KEY`.

- [ ] **Step 4: Run green test**

Run: `go test ./internal/config`

Expected: PASS.

## Task 2: Storage Models and Migration

**Files:**
- Create: `internal/storage/models.go`
- Create: `internal/storage/store.go`
- Create: `internal/storage/store_test.go`

- [ ] **Step 1: Write failing repository tests**

```go
func TestKnowledgeUpsertMarksChangedVectorPending(t *testing.T) {
    db := testDB(t)
    store := storage.NewStore(db)
    first := storage.KnowledgeEntry{SourceKey: "x", Keyword: "x", Answer: "old", Content: "old"}
    require.NoError(t, store.UpsertKnowledgeEntries(context.Background(), []storage.KnowledgeEntry{first}, 1))
    second := storage.KnowledgeEntry{SourceKey: "x", Keyword: "x", Answer: "new", Content: "new"}
    require.NoError(t, store.UpsertKnowledgeEntries(context.Background(), []storage.KnowledgeEntry{second}, 2))
    got, err := store.ListEnabledKnowledge(context.Background())
    require.NoError(t, err)
    require.Equal(t, "pending", got[0].VectorStatus)
}
```

- [ ] **Step 2: Run red test**

Run: `go test ./internal/storage`

Expected: FAIL because storage package does not exist.

- [ ] **Step 3: Implement GORM models and repositories**

Create models for `KnowledgeEntry`, `KnowledgeImportRun`, `Admin`, `Blacklist`, `ScheduledJob`, `ProcessedEvent`. Implement migrations, knowledge upsert, enabled-list loading, admins/blacklist CRUD, scheduled job CRUD, processed-event insert/check/cleanup.

- [ ] **Step 4: Run green test**

Run: `go test ./internal/storage`

Expected: PASS.

## Task 3: WPS Parser and RAG Enrichment

**Files:**
- Create: `internal/knowledge/types.go`
- Create: `internal/knowledge/parser.go`
- Create: `internal/knowledge/parser_test.go`

- [ ] **Step 1: Write failing parser tests**

```go
func TestParseRowsBuildsMenuPathAndIgnoresThirdColumn(t *testing.T) {
    rows := [][]string{
        {"%000", "交通\n\n%001 火车站", "维护备注"},
        {"%001", "请选择\n\n%0011 杭州东站", ""},
        {"%0011", "从朝晖去杭州东站路线", ""},
    }
    entries, report := knowledge.ParseRows(rows)
    require.Equal(t, 1, report.IgnoredNoteRows)
    leaf := findEntry(entries, "%0011")
    require.Equal(t, "朝晖校区交通 / 火车站 / 杭州东站", leaf.Path)
    require.Contains(t, leaf.Content, "杭州东站")
    require.Contains(t, leaf.Content, "从朝晖去杭州东站路线")
}
```

- [ ] **Step 2: Run red test**

Run: `go test ./internal/knowledge -run TestParseRows`

Expected: FAIL because parser is missing.

- [ ] **Step 3: Implement parser**

Implement row normalization, optional columns `aliases/category/usage/status/source_id`, third-column ignore, duplicate handling, `%编号` child extraction, path generation, chitchat detection, and generated `content`.

- [ ] **Step 4: Run green test**

Run: `go test ./internal/knowledge -run TestParseRows`

Expected: PASS.

## Task 4: Keyword Index, Cache, and Text Retriever

**Files:**
- Create: `internal/knowledge/index.go`
- Create: `internal/knowledge/retriever.go`
- Create: `internal/cache/cache.go`
- Create: `internal/knowledge/retriever_test.go`
- Create: `internal/cache/cache_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestKeywordIndexMatchesKeywordAndAlias(t *testing.T) {
    idx := knowledge.NewKeywordIndex([]knowledge.Entry{{Keyword: "选课", Aliases: []string{"怎么选课"}, Answer: "选课说明", Enabled: true, ExactReply: true}})
    got, ok := idx.Lookup(" 怎么选课 ")
    require.True(t, ok)
    require.Equal(t, "选课说明", got.Answer)
}
```

- [ ] **Step 2: Run red test**

Run: `go test ./internal/knowledge ./internal/cache`

Expected: FAIL because index/cache are missing.

- [ ] **Step 3: Implement index/cache/retriever**

Implement atomic index replacement, exact keyword/alias lookup, event TTL cache, exact/fulltext-like in-memory retriever for tests and repository-backed retrieval hooks.

- [ ] **Step 4: Run green test**

Run: `go test ./internal/knowledge ./internal/cache`

Expected: PASS.

## Task 5: AI RAG Service and Vector Interfaces

**Files:**
- Create: `internal/vector/vector.go`
- Create: `internal/ai/service.go`
- Create: `internal/ai/service_test.go`

- [ ] **Step 1: Write failing RAG tests**

```go
func TestServiceReturnsFixedMessageWhenNoDocuments(t *testing.T) {
    svc := ai.NewService(ai.Options{Retriever: ai.StaticRetriever{}, Chat: ai.StaticChat{}})
    got, err := svc.Answer(context.Background(), "不存在的问题")
    require.NoError(t, err)
    require.Equal(t, "知识库里没有找到相关内容", got)
}
```

- [ ] **Step 2: Run red test**

Run: `go test ./internal/ai ./internal/vector`

Expected: FAIL because AI/vector packages are missing.

- [ ] **Step 3: Implement RAG service**

Implement retriever and chat interfaces, prompt builder, no-result fixed reply, metadata formatting, OpenAI-compatible adapter placeholder behind interface, and vector noop/in-memory store.

- [ ] **Step 4: Run green test**

Run: `go test ./internal/ai ./internal/vector`

Expected: PASS.

## Task 6: Commands, Bot Pipeline, and Quote Client

**Files:**
- Create: `internal/commands/commands.go`
- Create: `internal/commands/admin.go`
- Create: `internal/bot/pipeline.go`
- Create: `internal/quote/client.go`
- Create: tests for each package

- [ ] **Step 1: Write failing command and pipeline tests**

```go
func TestPipelineFallsBackToKeywordReply(t *testing.T) {
    p := bot.NewPipeline(bot.Options{Knowledge: fakeKnowledge{reply: "我在！"}, Sender: &fakeSender{}})
    err := p.HandleGroupMessage(context.Background(), bot.GroupMessage{GroupID: 1, UserID: 2, Text: "精小弘"})
    require.NoError(t, err)
    require.Equal(t, "我在！", p.Sender.(*fakeSender).lastText)
}
```

- [ ] **Step 2: Run red test**

Run: `go test ./internal/commands ./internal/bot ./internal/quote`

Expected: FAIL because packages are missing.

- [ ] **Step 3: Implement commands and pipeline**

Implement `/test`, `/reload`, `/ai`, `/q`, admin CRUD commands, blacklist filtering, welcome message, quote request payload, scheduler command parsing.

- [ ] **Step 4: Run green test**

Run: `go test ./internal/commands ./internal/bot ./internal/quote`

Expected: PASS.

## Task 7: NapCat Adapter, Scheduler, App Wiring, Deployment

**Files:**
- Create: `internal/napcat/adapter.go`
- Create: `internal/scheduler/scheduler.go`
- Create: `cmd/bot/main.go`
- Create: `Dockerfile`, `compose.yaml`, `README.md`

- [ ] **Step 1: Write adapter/scheduler tests**

```go
func TestSchedulerRunsSingleJobOnce(t *testing.T) {
    var sent int
    runner := scheduler.New(scheduler.Options{Send: func(context.Context, int64, string) error { sent++; return nil }})
    runner.AddForTest(scheduler.Job{Type: "单次", GroupID: 1, Message: "hello", RunAt: time.Now().Add(-time.Second)})
    runner.RunDue(context.Background(), time.Now())
    runner.RunDue(context.Background(), time.Now())
    require.Equal(t, 1, sent)
}
```

- [ ] **Step 2: Run red test**

Run: `go test ./internal/napcat ./internal/scheduler ./cmd/bot`

Expected: FAIL because packages are missing.

- [ ] **Step 3: Implement app wiring**

Implement NapCat reverse WS server wrapper, SDK API sender, scheduler runtime, main startup, graceful shutdown, config example, Dockerfile, compose file, and README runbook.

- [ ] **Step 4: Run final verification**

Run:

```bash
go test ./...
go test ./... -run TestParseRowsBuildsMenuPathAndIgnoresThirdColumn
go test ./... -run TestServiceReturnsFixedMessageWhenNoDocuments
```

Expected: all tests PASS.

## Self-Review

- Spec coverage: tasks cover config, storage, WPS import, third-column ignore, menu tree parsing, keyword reply, `/reload`, `/ai`, quote, admin/blacklist, scheduler, NapCat adapter, deployment docs.
- Placeholder scan: no `TBD` or implementation-later placeholders.
- Type consistency: plan uses `knowledge.Entry`, `storage.KnowledgeEntry`, and AI/vector interfaces consistently with package ownership.
