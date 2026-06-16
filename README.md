# Jxh-Go

精小弘 Go + NapCat + Eino 重构实现。

## 当前能力

- NapCat OneBot v11 反向 WebSocket 接入。
- WPS `release` sheet 两列回复表导入。
- 第三列维护备注不入库、不参与 RAG。
- `%编号` 菜单树解析，生成 `path` 和 RAG `content`。
- 关键词与 aliases 精确回复。
- `/reload` 同步知识库并刷新关键词缓存和 `/ai` retriever。
- `/ai <问题>` 基于知识库检索回答；没有配置模型时使用抽取式 fallback。
- 管理员、黑名单、定时任务、processed events、知识库表的 GORM 模型。
- `processed_events` 支持按时间清理。

## 本地运行

```bash
cp config.example.yaml config.yaml
go test ./...
go run ./cmd/bot -config config.yaml
```

NapCat 反向 WebSocket 地址：

```text
ws://bot:8080/onebot/v11/ws
```

## Docker Compose

```bash
cp config.example.yaml config.yaml
docker compose up --build
```

默认只启动 `mysql` 和 `bot`。引用图服务可按实际镜像调整后启用 `quote` profile。

## WPS 表规则

基础列：

| 列 | 字段 |
| --- | --- |
| A | keyword |
| B | answer |

C 列如果存在，只作为维护备注，不进入数据库、向量索引或 `/ai` prompt。

可选列：

| 列 | 字段 |
| --- | --- |
| D | aliases |
| E | category |
| F | usage |
| G | status |
| H | source_id |
