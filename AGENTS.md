# AGENTS.md — Sub2API 二开分支维护手册

## 项目身份

这是 Sub2API 的二开 fork 工作区，不是上游原始仓库。

- 当前工作目录：`/home/div/1_Project_dir/AI/sub2api`
- 我们的仓库：`origin = https://github.com/yunqiasen/sub2api.git`
- 上游仓库：`upstream = https://github.com/Wei-Shaw/sub2api.git`
- 当前二开分支：`sub2-fork`
- 上游主线：`upstream/main`
- 不要向 `upstream` 推送；`upstream` 只用于拉取更新。

`docs/superpowers` 里的 WSL Ops Panel 规划不是 Sub2API 二开规划。它是另一个 WSL 运维面板项目，只把 Sub2API 当作被管理的 Docker/systemd 对象。不要把它当成本仓库功能路线图。

`session_context_*.md` 是本地会话记忆。可以参考，但代码、Git 历史、运行容器和数据库状态优先级更高。

## 运行目录与数据边界

本项目已经收敛为单目录维护：源码、Docker compose、配置和运行挂载都在 `/home/div/1_Project_dir/AI/sub2api`。

当前 compose 约定：

- 应用容器：`sub2api-fork`
- PostgreSQL：`sub2api-fork-postgres`
- Redis：`sub2api-fork-redis`
- 本地镜像名：`local/sub2api:sub2-fork`
- 主要持久化目录：`data/`、`postgres_data/`、`redis_data/`、`logs/`
- 本地环境文件：`.env`

不要删除、重建、提交或随意移动这些运行数据目录和 `.env`。需要清理或迁移前，先确认容器状态和备份路径。

常用检查：

```bash
git status --short --branch
git remote -v
docker compose ps
docker compose logs --tail=120 sub2api
```

健康检查优先看实际运行态，不要只看前端提示或历史文档。

## 分支维护规则

日常开发只在 `sub2-fork` 上做。`main` 用来跟上游主线保持同步，不放二开代码。

推荐同步流程：

```bash
git fetch origin
git fetch upstream

git switch main
git merge --ff-only upstream/main
git push origin main

git switch sub2-fork
git merge --no-ff main
```

如果 `main` 无法快进，先查差异，不要直接 `reset --hard`。如果用户明确要求把 `main` 重置成上游镜像，才可在确认无本地价值改动后执行。

合并上游进 `sub2-fork` 时，按这个顺序判断冲突：

1. 先保留上游安全修复、协议兼容、模型映射、计费和调度修复。
2. 再保留我们二开的后台体验、日志审计、账号分类、Docker/CI 兼容改动。
3. 冲突文件逐个看，不要整目录用 ours/theirs 覆盖。
4. 合并后跑针对测试，再做一次容器构建或启动验证。

上游大版本合并前，建议先停容器：

```bash
docker compose down
```

涉及数据库迁移时，先备份 `postgres_data/` 或导出数据库，再启动新版本。

## 当前二开内容

已确认的二开改动主要有三类：

1. 账号错误状态细分筛选
   - 后端：`backend/internal/repository/account_error_filters.go`
   - 前端：`frontend/src/components/admin/account/AccountTableFilters.vue`
   - 目标：让账号页能按更细的错误状态筛选。

2. 使用记录提示词审计
   - 迁移：`backend/migrations/145_usage_log_request_prompt.sql`
   - 提取：`backend/internal/service/request_prompt.go`
   - 展示：`frontend/src/components/admin/usage/UsageTable.vue`
   - 边界：这是 `usage_logs.request_prompt` 摘要级记录，不等于完整请求体保存。

3. Docker / CI 交付兼容
   - `Dockerfile` healthcheck 禁用代理，避免本地代理影响 `/health`。
   - `docker-compose.yml` 使用当前目录构建并挂载运行数据。
   - `.github/workflows/docker-build.yml` 用于 `sub2-fork` 推送后构建 GHCR 镜像。

规划中的请求审计二开要优先兼容现有使用记录体系：成功请求扩展 `usage_logs`，失败请求沿用并增强 `ops_error_logs`。不要照抄 CPA 新建独立日志中心；除非后续实测表膨胀或查询压力无法承受，否则不新增主日志表。

## 二开开发原则

保持二开补丁小而清楚。能独立成提交的，不要揉进上游同步提交里。

建议提交类型：

```bash
feat: add request capture logs
fix: keep healthcheck proxy-safe
chore: merge upstream 0.1.xxx into sub2-fork
ci: build sub2-fork image to GHCR
```

不要大面积格式化上游文件。不要把运行配置、账号凭证、导出凭证、日志原文提交到 Git。

遇到高漂移信息，写 live-check 口径，不写死。例如域名、端口、容器健康、账号数量、usage 数量、任务成功状态，都以接口、数据库或容器实时结果为准。

## 后端维护

后端是 Go + Gin + Ent + Wire。

常用命令：

```bash
cd backend
go test ./...
go test -tags=unit ./...
go test -tags=integration ./...
golangci-lint run ./...
```

改 `backend/ent/schema/*.go` 后必须重新生成并提交生成文件：

```bash
cd backend
go generate ./ent
go generate ./cmd/server
```

新增数据库字段要走 migration，不要只改 Ent schema。迁移编号不要和现有文件冲突。

请求链路改动优先查这些位置：

- OpenAI/Responses：`backend/internal/handler/openai_gateway_handler.go`
- 通用 gateway：`backend/internal/handler/gateway_handler.go`
- 使用记录：`backend/internal/service/usage_log.go`
- request prompt 摘要：`backend/internal/service/request_prompt.go`
- 错误记录：`backend/internal/handler/ops_error_logger.go`
- Ops 详情：`backend/internal/service/ops_request_details.go`

## 前端维护

前端是 Vue 3 + TypeScript + pnpm。不要用 npm 改锁文件。

常用命令：

```bash
pnpm --dir frontend install
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run test:run
pnpm --dir frontend run build
```

后台页面主要在：

- 账号页：`frontend/src/views/admin/AccountsView.vue`
- 使用记录：`frontend/src/views/admin/UsageView.vue`
- 使用记录表格：`frontend/src/components/admin/usage/UsageTable.vue`
- Ops 错误：`frontend/src/views/admin/ops/`
- 管理 API：`frontend/src/api/admin/`

前端按钮的 success 只代表请求返回成功，不代表后台任务已真正完成。批量刷新、导出、同步这类功能必须有后台任务状态或后续读取验证。

## Docker 和 GHCR

本地开发默认使用：

```bash
docker compose build sub2api
docker compose up -d
docker compose ps
```

`.github/workflows/docker-build.yml` 目标是在 `sub2-fork` 推送后构建：

- `ghcr.io/yunqiasen/sub2api:sub2-fork`
- `ghcr.io/yunqiasen/sub2api:<VERSION>-sub2-fork`
- `ghcr.io/yunqiasen/sub2api:sha-<short_sha>`

如果推送 workflow 文件被拒绝，原因通常是 GitHub token 缺少 `workflow` scope。不要用绕过方式乱推；换带权限的 token 后再推。

服务器部署二开镜像时，服务器只挂载配置、数据库、Redis、日志等运行数据，不热挂载整套源码。源码热挂载只适合本地开发工作区。

## 管理 API 与批量操作

仓库内有专用管理 CLI：`skills/sub2api-admin`。账号、凭证、兑换码、代理、错误规则、TLS 模板、批量导入导出等操作优先用这个 CLI，不要临时拼散乱 curl。

导出账号或凭证时写入文件，不要把 token、凭证或完整导出内容贴到聊天里。

批量删除、批量刷新、批量清错前先列出目标 ID 和名称。执行后用只读接口复查。

## 验证口径

完成代码修改前，至少做对应层验证：

- 后端改动：目标 `go test`，必要时跑 `go test ./...`。
- Ent schema 或 migration：跑生成命令和 migration/schema 相关测试。
- 前端改动：跑相关 Vitest、`lint:check`、`typecheck`。
- Docker/部署改动：跑 `docker compose build sub2api` 和 `docker compose up -d`。
- 管理后台行为：用浏览器或管理 API 验证实际返回和落库结果。

验证结果写清楚命令和结论。不能把“代码看起来对”当作完成。
