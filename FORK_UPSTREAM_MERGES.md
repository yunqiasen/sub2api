# 二开分支上游合并记录

此文件只记录 `upstream/main -> main -> sub2-fork` 的合并结果、二开兼容修复和验证证据。长期维护规则以仓库根目录 `AGENTS.md` 为准。

## 2026-09-25：0.1.165 -> 0.2.8

- 上游目标：`upstream/main` / `a3eb7ef30` / `0.2.8`，相对旧主线增加 1979 个提交。
- 本地 `main` 通过 `--ff-only` 同步；在同一目录切回 `sub2-fork` 合并，未创建 worktree。
- 合并前二开提交：`434c41df8`；逐处处理 37 个冲突文件，保留上游协议、计费、调度与安全修复。
- 本次仅代码同步和构建验证，不部署本地数据/VPS，不启动原有已停止的应用与数据库容器。

### 上游重点变化

- 更新模型与平台接入、OpenCode 用量窗口、Responses/Chat/Anthropic 协议转换和工具调用兼容。
- 保留断流已观测 token 的记账、最终 reasoning effort、图像/视频实际用量与新计费字段。
- 引入账号跨页选择、母/影子账号管理与有限并发批量删除；保留代理回退、配额刷新等修复。
- 保留 OAuth pending exchange 账号接管防护、流式错误处理、备份与迁移锁等上游修复。

### 二开兼容与回归修复

- 成功审计仍写 `usage_logs`，失败审计仍写 `ops_error_logs`，不增加独立日志主表。
- 审计字段重新接入上游抽出的记账闭包和仓储文件；INSERT 参数为 78 个，批量/best-effort/查询扫描保持一致。
- 重新生成 Ent/Wire；保留 system/developer 提示词、请求摘要、输出文本、实际工具调用与流式聚合。
- 修复 partial stream usage 漏传响应审计内容；保留 failover 返回空 result 的规则，避免重复计费。
- 保留按组删除、内置“全部账号”删除、2000 分页与上游跨页全选。选中批量删除路由使用上游 BatchDelete。
- 修复母账号级联删除越过分组范围的问题：范围外有影子账号时保留母账号并报告失败；范围内影子账号只删除、计数一次。
- 补齐中英文二开审计/批量删除文案，修复新版 i18n 完整性检查发现的缺键。
- 新增断流审计、跨组影子保护、影子去重、未选中行时组删除按钮、审计与上游字段共同落库回归测试。
- `AGENTS.md` 写明单工作区切分支、仅代码同步不动服务、pnpm 9 与测试代理边界。

### 验证

- `go generate ./ent`、`go generate ./cmd/server`：完成。
- `go test ./... -count=1`、`go test -tags=unit ./... -count=1`、`go test -tags=integration ./... -count=1`：全量通过。
- `golangci-lint run ./... --timeout=30m`（2.13.0）：0 issues。
- PostgreSQL 18.1 / Redis 8.4 独立容器：迁移及仓储集成测试通过，新增审计字段与上游字段的共同写入/读取测试通过。
- `pnpm --dir frontend run lint:check`：通过；Vitest 333 个文件、2489 个测试通过。
- `pnpm --dir frontend run build`：通过，含 TypeScript 和 i18n 检查；保留上游 Browserslist/大 chunk 警告。
- Docker Compose 构建成功。构建时仅通过 `--build-arg HTTP_PROXY/HTTPS_PROXY=http://172.17.0.1:7890` 使用现有代理，未修改 daemon/运行配置。
- 镜像内 `/app/sub2api -version` 返回 `0.2.8`；`pg_dump --version` 正常。
- 临时应用 + PostgreSQL + Redis 完整启动冒烟通过，`/health` 返回 `{"status":"ok"}`；数据库实际包含 `usage_logs` 79 列、`ops_error_logs` 64 列。临时容器/网络随后删除，未挂载生产目录。
- Linux 下 Compose 安全/网关环境/运行资源/simple mode 和 Caddy 缓存脚本测试通过。macOS 专用 Apple 容器测试依赖 BSD `stat`，交由现有 macOS CI 验证，本机仅通过语法检查。
- `git diff --cached main --check`：二开差异无空白错误。完整合并范围内上游自带的提示词/Markdown 尾随空格按原文保留，不批量格式化上游文件。

## 2026-07-26：0.1.138 -> 0.1.165

- 上游目标：`upstream/main` / `2730c1c43` / `0.1.165`
- 我们的主线：`main` 已快进到同一提交并同步至 `origin/main`
- 二开目标：把 `main` 合并进 `sub2-fork`，保留现有二开功能
- 上游跨度：1298 个提交，其中 820 个非合并提交

### 上游重点变化

- 新增 ChatGPT Live 网关、Claude Opus 5、Grok/复合路由、批量图片等能力。
- 修复注册邮箱别名查重绕过、误拒、无界扫描和并发竞态。
- 修复 PostgreSQL 16 及以下版本的 Ollama 用量刷新失效。
- 修复 OpenAI/Grok/Gemini 的 Responses、工具调用、图像输出、池模式重试和计费兼容问题。
- 修复优雅关停时可能跳过 Cleanup，导致缓冲用量或计费记录丢失的问题。
- 修复客户端 IP、可信代理、Docker 反代环境下的解析与审计一致性问题。
- 升级 PostCSS、Axios、`golang.org/x/text` 等依赖，处理已知安全公告。

### 二开兼容修复

- 保留账号错误状态细分筛选、按分组删除全部账号、全部账号组删除和单页最多 2000 条。
- 保留成功请求写入 `usage_logs`、失败请求写入 `ops_error_logs` 的审计体系。
- 将审计字段重新接入上游重写后的 usage log 单条、批量、best-effort 插入和查询 SQL。
- 为 OpenAI、Anthropic、Responses、Chat Completions 的流式与非流式成功响应补齐输出文本、摘要和实际工具调用采集。
- 重新生成 Ent/Wire 文件，并更新受上游接口变化影响的后端断言与前端 mock。
- 清理合并带入的两处空白格式问题；源冻结补丁文件中的历史空白保持原样。
- 修正上游 `unit` 构建标签下仍按旧参数数量和索引断言的两个仓储测试，避免 CI 专用测试路径漏报。

### 验证

```bash
cd backend
go generate ./ent
go generate ./cmd/server
go test ./... -count=1
go test -tags=unit ./... -count=1
golangci-lint run ./... --timeout=30m

pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend exec vitest run --reporter=dot
pnpm --dir frontend run build
```

结果：

- 后端普通构建标签和 `unit` 构建标签的全量测试均通过。
- golangci-lint 2.9.0：0 issues。
- 前端 ESLint、TypeScript 检查通过。
- 前端 190 个测试文件、1306 个测试全部通过。
- 前端生产构建通过；保留现有 Browserslist、动态导入和大 chunk 警告。
