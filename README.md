<p align="center">
  <h1 align="center">精小弘 Jxh-Go</h1>
  <p align="center">基于 Go、NapCat 和 Eino 的精弘 QQ 群助手</p>
</p>

<p align="center">
  <a href="https://github.com/cloudwego/eino"><img alt="Eino" src="https://img.shields.io/badge/Eino-Agent-blue?style=flat-square"></a>
  <a href="https://github.com/NapNeko/NapCatQQ"><img alt="NapCat" src="https://img.shields.io/badge/NapCat-OneBot11-green?style=flat-square"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go">
  <img alt="MySQL" src="https://img.shields.io/badge/MySQL-8.4+-4479A1?style=flat-square&logo=mysql&logoColor=white">
</p>

## 简介

Jxh-Go 是精弘 QQ 群助手的 Go 重构版本，面向浙江工业大学相关 QQ 群的自动问答、知识库回复和群管理场景。

它通过 NapCat 接入 OneBot 11，用 MySQL 保存知识库和运行状态，并用 Eino 接入 `/ai`。同一份 WPS 回复表既可以做关键词精确回复，也可以作为 AI 检索问答的知识源。

## 主要能力

- **关键词回复**：从 WPS 回复表导入 `keyword`、`answer` 和 `aliases`，在群聊中精确匹配。
- **菜单问答**：兼容 `%编号` 菜单树，导入时生成路径，方便回复和检索。
- **AI 问答**：`/ai <问题>` 基于知识库检索回答；未配置模型时使用抽取式 fallback。
- **群管理**：支持管理员、黑名单、禁言、NapCat 重启和定时任务。
- **引用图**：回复消息后发送 `/q`，调用 quote 服务生成引用图。
- **事件去重**：记录已处理事件，降低 NapCat 重连时重复响应的概率。

## 快速开始

### 1. 准备依赖

本地需要 Docker Compose。`docker-compose.yaml` 现在会一次启动 MySQL、NapCat、引用图服务和 bot。

### 2. 复制配置

```bash
cp config.example.yaml config.yaml
```

先重点检查这些配置：

- `onebot.access_token`：必须和 NapCat WebSocket token 一致。
- `wps.share_url`：WPS 导出文档链接；为空时不会自动同步知识库。
- `wps.sid`：受保护 WPS 文档需要填写，也可用 `JXH_WPS_SID` 注入。
- `database.password`：默认匹配 compose 的 `jxh_password`。
- `ai.api_key`、`ai.model`：可选；为空时 `/ai` 使用抽取式 fallback。

### 3. 启动全部服务

```bash
make compose-up
```

等价命令：

```bash
NAPCAT_UID=$(id -u) NAPCAT_GID=$(id -g) docker compose up -d --build
```

compose 会同时启动 MySQL、NapCat、quote 和 bot。

持久化数据默认放在仓库根目录的 `./data/` 下，便于直接打包备份和迁移。

### 4. 配置 NapCat

打开 WebUI：

```text
http://127.0.0.1:6099/webui
```

WebUI token 可通过日志查看：

```bash
docker logs napcat
```

登录 QQ 后，开启 OneBot 11 正向 WebSocket：

- 监听地址：`0.0.0.0`
- 监听端口：`3001`
- token：和 `config.yaml` 的 `onebot.access_token` 一致

NapCat 运行在容器内，监听地址不要填 `127.0.0.1`，否则宿主机上的 bot 会连不上。

### 5. 启动 bot

如果你用仓库里的 compose，这一步已经包含在 `make compose-up` 里了，不需要单独再起 bot。

```bash
make run
```

等价命令：

```bash
go run ./cmd/bot -config config.yaml
```

启动后在 QQ 群里发送 `/test`。如果返回 `精小弘正常`，说明接入成功。配置好 WPS 后，发送 `/reload` 导入知识库。

## WPS 知识表

`wps.share_url` 应填写网页端“右键文件 -> 导出文档链接”得到的链接，或可直接下载的 `.xlsx` 地址。

普通 `365.kdocs.cn/l/...` 分享页通常返回 HTML 页面，不能直接导入。受保护文档需要配置 `wps.sid` 或环境变量 `JXH_WPS_SID`。

基础列：

| 列 | 字段 | 说明 |
| --- | --- | --- |
| A | `keyword` | 关键词 |
| B | `answer` | 标准回答 |
| C | 维护备注 | 不入库，不参与回复或 AI 检索 |

可选列：

| 列 | 字段 | 说明 |
| --- | --- | --- |
| D | `aliases` | 同义问法，多个用分隔符隔开 |
| E | `category` | 分类 |
| F | `usage` | 用途控制 |
| G | `status` | 启用状态 |
| H | `source_id` | 稳定 ID，修改 keyword 时用于保留同一条记录 |

导入器会解析 `%编号` 菜单树，并生成 `path` 和 AI 检索用的 `content`。

## 常用命令

| 命令 | 说明 |
| --- | --- |
| `/test` | 连通性测试 |
| `/reload` | 从 WPS 同步知识库，并刷新缓存 |
| `/ai <问题>` | 基于知识库检索回答 |
| `/q` | 回复一条消息后生成引用图 |
| `/admin restart` | 请求 NapCat 重启 |
| `/admin ban <时长>` | 禁言被 @ 的用户；时长支持 `10m`、`1h` 或秒数 |

管理员中文子命令：

| 命令 | 说明 |
| --- | --- |
| `/admin 添加管理员 @用户` | 添加管理员 |
| `/admin 移除管理员 @用户` | 移除管理员 |
| `/admin 所有管理员` | 查看管理员 |
| `/admin 添加黑名单 @用户` | 添加黑名单 |
| `/admin 移除黑名单 @用户` | 移除黑名单 |
| `/admin 所有黑名单` | 查看黑名单 |
| `/admin 定时任务 查看` | 查看定时任务 |
| `/admin 定时任务 添加 <每天|单次> <HH:MM> <群聊ID> <消息内容>` | 添加定时任务 |
| `/admin 定时任务 移除 <任务ID>` | 移除定时任务 |

群主天然拥有管理员权限；普通管理员信息保存在 MySQL。

## 配置和环境变量

主配置文件是 `config.yaml`。示例配置在 `config.example.yaml`，字段说明写在注释里。

常用环境变量：

| 环境变量 | 作用 |
| --- | --- |
| `JXH_ONEBOT_TOKEN` | OneBot WebSocket token |
| `JXH_ONEBOT_WS_URL` | NapCat 正向 WebSocket 地址 |
| `JXH_WPS_SID` | WPS 登录态 sid |
| `JXH_WPS_TIMEOUT_SEC` | WPS 请求超时时间 |
| `MYSQL_DATABASE` | MySQL 数据库名，compose 部署使用 |
| `MYSQL_USER` | MySQL 用户名，compose 部署使用 |
| `MYSQL_PASSWORD` | MySQL 密码，compose 部署使用 |
| `MYSQL_ROOT_PASSWORD` | MySQL root 密码，compose 部署使用 |
| `JXH_MYSQL_PASSWORD` | bot 直连运行时的 MySQL 密码；compose 部署通常用 `MYSQL_PASSWORD` |
| `JXH_MYSQL_DSN` | 完整 MySQL DSN，设置后优先使用 |
| `JXH_AI_PROVIDER` | ChatModel 提供方，支持 `openai`、`ark` |
| `JXH_AI_BASE_URL` | ChatModel base URL |
| `JXH_AI_API_KEY` | ChatModel API Key |
| `JXH_AI_MODEL` | ChatModel 模型名；openai 填模型名，ark 填方舟推理接入点 ID |

AI 行为：

- `ai.enabled: false`：`/ai` 返回未启用。
- 未配置 `ai.api_key` 或 `ai.model`：使用抽取式 fallback。
- `ai.provider: ark` 时，`ai.model` 填方舟推理接入点 ID，例如 `ep-xxxxxxxx`。

## 引用图服务

引用图服务默认不启动。需要 `/q` 时执行：

```bash
docker compose --profile quote up -d quote
```

compose 内 bot 对应配置：

```yaml
quote:
  base_url: "http://quote:5000"
```

## 数据库和代码生成

项目采用 schema-first，运行时不使用 `AutoMigrate`。表结构以 `deploy/mysql/init/001_schema.sql` 为准。

MySQL 首次初始化时会自动执行该 SQL。若 `./data/mysql` 目录里已经有旧数据，初始化 SQL 不会重复执行。

需要重建空库时：

```bash
docker compose down
rm -rf ./data/mysql
docker compose up -d mysql
```

重新生成 GORM query/model：

```bash
make gormgen-install
export JXH_GORMGEN_DSN="jxh:jxh_password@tcp(127.0.0.1:3306)/jxh_bot?charset=utf8mb4&parseTime=True&loc=Local"
make gormgen
```

更多说明见 `docs/storage-gormgen.md`。

## 开发命令

```bash
make help          # 查看所有 make target
make run           # 本地运行 bot
make build         # 构建 bin/jxh-go
make test          # 运行测试
make fmt           # go fmt ./...
make compose-up    # 启动 mysql 和 napcat
make compose-logs  # 查看 compose 日志
```

## 目录结构

| 路径 | 说明 |
| --- | --- |
| `cmd/bot` | bot 启动入口 |
| `internal/config` | 配置加载、默认值和环境变量覆盖 |
| `internal/cache` | 关键词索引和事件去重的内存缓存 |
| `internal/bot` | 群消息处理管线和命令路由 |
| `internal/commands` | 管理员、黑名单、定时任务命令 |
| `internal/knowledge` | WPS 解析、关键词索引、文本检索 |
| `internal/ai` | `/ai` RAG 服务和 Eino ChatModel 适配 |
| `internal/storage` | GORM repository、业务存储模型和 generated query/model |
| `internal/napcat` | NapCat SDK 适配层 |
| `internal/quote` | 引用图请求和消息内容转换 |
| `internal/scheduler` | 定时任务运行时 |
| `internal/vector` | 向量检索预留目录，当前未放置实现文件 |
| `deploy/mysql/init` | MySQL 初始化 SQL |
| `docs` | 设计文档、实现计划和 GORM Gen 说明 |
| `scripts` | 代码生成和工具安装脚本 |
| `data/` | MySQL、NapCat、bot 和 WPS 缓存的持久化根目录 |
| `Dockerfile` | bot 容器镜像构建文件 |
| `docker-compose.yaml` | MySQL、NapCat、quote 和 bot 的完整 compose |
