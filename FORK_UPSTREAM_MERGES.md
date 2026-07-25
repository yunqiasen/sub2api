# 二开分支上游合并记录

此文件只记录 `upstream/main -> main -> sub2-fork` 的合并结果、二开兼容修复和验证证据。长期维护规则以仓库根目录 `AGENTS.md` 为准。

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

### 验证

```bash
cd backend
go generate ./ent
go generate ./cmd/server
go test ./... -count=1
golangci-lint run ./... --timeout=30m

pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend exec vitest run --reporter=dot
pnpm --dir frontend run build
```

结果：

- 后端全量测试通过。
- golangci-lint 2.9.0：0 issues。
- 前端 ESLint、TypeScript 检查通过。
- 前端 190 个测试文件、1306 个测试全部通过。
- 前端生产构建通过；保留现有 Browserslist、动态导入和大 chunk 警告。
