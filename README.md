# Jxh-Go

精小弘 Go + NapCat + Eino 重构实现。

## 当前能力

- NapCat OneBot v11 WebSocket 接入，默认采用 MumuBot 类似的正向 WebSocket：bot 主动连接 NapCat。
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
NAPCAT_UID=$(id -u) NAPCAT_GID=$(id -g) docker compose up -d mysql napcat
go test ./...
go run ./cmd/bot -config config.yaml
```

NapCat 由 compose 作为外部依赖启动，Go bot 服务单独运行。启动后打开 NapCat WebUI：

```text
http://127.0.0.1:6099/webui
```

WebUI 登录 token 可通过容器日志查看：

```bash
docker logs napcat
```

在 NapCat WebUI 中登录 QQ，并开启 OneBot 11 正向 WebSocket，监听端口使用 `3001`。bot 配置里保持：

```text
onebot.ws_url: ws://127.0.0.1:3001
```

如果修改了 NapCat 端口，需要同步修改 `config.yaml` 里的 `onebot.ws_url`。

## Docker Compose

`compose.yaml` 只用于启动外部依赖，不运行 bot 服务本身。

```bash
cp config.example.yaml config.yaml
NAPCAT_UID=$(id -u) NAPCAT_GID=$(id -g) docker compose up -d mysql napcat
go run ./cmd/bot -config config.yaml
```

compose 默认启动：

| 服务 | 作用 | 暴露端口 |
| --- | --- | --- |
| `mysql` | GORM/MySQL 数据库 | `3306` |
| `napcat` | QQ 登录和 OneBot 11 协议适配 | `3000`, `3001`, `6099` |

引用图服务可按实际镜像调整后启用 `quote` profile：

```bash
docker compose --profile quote up -d
```

Go bot 不进 compose，保持单独运行：

```bash
go run ./cmd/bot -config config.yaml
```

NapCat 数据通过 Docker volume 持久化：

| volume | 容器路径 | 用途 |
| --- | --- | --- |
| `napcat_qq` | `/app/.config/QQ` | QQ 登录态 |
| `napcat_config` | `/app/napcat/config` | NapCat 配置 |
| `napcat_plugins` | `/app/napcat/plugins` | NapCat 插件 |

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
