# Sub2API 使用记录审计增强设计

## 目标

在现有 Sub2API 使用记录体系上做二开增强，让管理员能在“使用记录”里看清每次请求的关键上下文：用户提示词、系统/开发者 instructions、可用工具、实际工具调用、输出摘要、错误关联、IP 与导出内容。

这不是照抄 CPA 的独立请求日志中心。Sub2API 已经有 `usage_logs`、`ops_error_logs`、Usage 管理页和二开过的 `request_prompt` 雏形，本设计只做兼容增强。

## 当前基础

现有使用记录列表已经有这些列：

```text
用户 / API 密钥 / 账户 / 模型 / 端点 / 分组 / 类型 / 计费模式 / TOKEN / 费用 / 首 TOKEN / 耗时 / 时间 / IP
```

当前二开已增加：

- `usage_logs.request_prompt`
- `backend/internal/service/request_prompt.go`
- 使用记录表格的 `request_prompt` 摘要和完整弹窗
- 使用记录导出里的 `request_prompt`

当前边界：

- `usage_logs` 负责成功请求、计费、token、耗时和基础排查。
- `ops_error_logs` 负责失败请求、上游错误和错误详情。
- 历史数据没有完整请求体，不能倒推出历史系统词、tools、tool calls。
- 新增审计字段只保证上线后的新请求可保存。

## 设计原则

1. 不新增独立主日志表。
2. 不破坏现有 usage 统计和计费逻辑。
3. 成功请求的审计字段保存在 `usage_logs`。
4. 失败请求的审计字段保存在 `ops_error_logs` 或其详情结构。
5. 前端继续使用 Usage 页面，不新增“请求日志中心”菜单。
6. 大字段必须截断、标记截断状态、保留 hash，避免数据库被无限撑大。
7. 敏感字段必须脱敏，不保存 Authorization、Cookie、完整 API Key、完整账号 token。

## 数据保存设计

### `usage_logs` 增强字段

成功请求继续保存到同一张表。新增字段建议如下：

| 字段 | 类型建议 | 用途 |
|---|---|---|
| `system_prompt_summary` | TEXT | 系统/开发者上下文摘要，列表展示 |
| `system_prompt_text` | TEXT | 系统/开发者/instructions 全文，详情展示，按上限截断 |
| `developer_prompt_text` | TEXT | developer 角色或开发者提示词，按上限截断 |
| `tool_names` | JSONB | 请求体 tools 中的可用工具名列表 |
| `tool_call_names` | JSONB | 响应流里实际调用过的工具名列表 |
| `tool_calls_json` | JSONB | 实际工具调用的轻量结构，不保存大参数全文 |
| `output_summary` | TEXT | 输出摘要，列表和导出使用 |
| `output_text` | TEXT | 输出文本片段，按上限截断 |
| `request_body_sha256` | VARCHAR(64) | 原始请求体 hash，用于去重和追踪 |
| `request_body_bytes` | INTEGER | 请求体字节数 |
| `request_body_truncated` | BOOLEAN | 请求体相关文本是否截断 |
| `response_text_truncated` | BOOLEAN | 输出文本是否截断 |
| `turn_metadata` | JSONB | `X-Codex-Turn-Metadata` 里的 session/thread/turn 信息 |
| `ip_location` | JSONB | IP 归属地缓存，如国家、省市、ASN、来源 |
| `audit_capture_version` | SMALLINT | 审计提取版本，便于以后兼容 |

`request_prompt` 保留，不改名。它继续代表用户提示词，可从 Chat Completions messages、Responses input、prompt 字段中提取。

### `ops_error_logs` 增强字段

失败请求不塞进 `usage_logs`。它们继续保存在 `ops_error_logs`，补齐与成功请求相同的审计字段：

- `request_prompt`
- `system_prompt_summary`
- `system_prompt_text`
- `developer_prompt_text`
- `tool_names`
- `tool_call_names`
- `output_summary`
- `request_body_sha256`
- `request_body_bytes`
- `request_body_truncated`
- `turn_metadata`
- `ip_location`
- `audit_capture_version`

错误响应继续用现有 error body / upstream error 字段，不重复存一份。

### 大字段上限

默认上限建议：

- 用户提示词：64 KB
- system/developer/instructions：128 KB
- 输出文本：64 KB
- tool_calls_json：32 KB
- 单行审计新增总量：不超过 512 KB

超过上限时截断，同时保存：

- 原始字节数
- sha256
- `*_truncated = true`

这样能排查问题，又不会把数据库当文本日志仓库无限堆。

## 采集链路

### 请求阶段

在现有 gateway handler 读取请求体后，提取审计信息：

- 用户提示词：复用并扩展 `ExtractRequestPrompt`
- system/developer/instructions：从 `messages`、`input`、`instructions` 字段提取
- tools：从 `tools`、`functions`、MCP 风格工具名中提取
- Codex turn：从 `X-Codex-Turn-Metadata` 请求头提取
- IP：沿用当前 `ip_address`，同时记录 IP 来源和归属地缓存

提取器做成独立服务，例如：

```text
backend/internal/service/usage_audit_extract.go
```

它只负责把请求体、响应事件、headers 转成结构化审计对象，不负责写库。

### 响应阶段

非流式响应：

- 从 JSON 响应中提取 output text
- 提取工具调用结构
- 生成输出摘要

流式/SSE 响应：

- 在转发流事件时轻量监听
- 聚合文本 delta 到上限
- 捕获 `response.output_item.added`、`response.function_call_arguments.done`、`response.custom_tool_call_input.done` 等工具调用事件
- 不阻塞客户端流式返回

### 写库阶段

成功请求：把审计对象随现有 usage log 异步写入 `usage_logs`。

失败请求：把审计对象随现有 ops error logger 写入 `ops_error_logs`。

如果审计提取失败，不影响请求转发和计费落库，只记录提取错误到内部日志。

## 前端设计

### 使用记录列表

保留当前列表结构，不做 CPA 式新页面。

建议新增列：

- `提示词`：默认显示，沿用现有 `request_prompt` 摘要。
- `实际调用`：默认隐藏，显示实际工具调用名摘要。
- `系统词`：默认隐藏，显示 system summary。
- `输出`：默认隐藏，显示 output summary。
- `操作`：默认显示，包含“预览”。

列表上只放摘要，不塞全文。

### 预览弹窗

每行右侧增加或强化“预览”。弹窗分块：

1. 基础信息
   - 用户、API Key、账户、模型、端点、分组、类型、计费、token、费用、耗时、时间、IP、归属地。
2. 用户提示词
   - `request_prompt` 全文。
3. 系统上下文
   - system/developer/instructions 摘要和全文。
4. 工具信息
   - 可用工具列表。
   - 实际调用工具列表。
   - 工具调用轻量结构。
5. 输出信息
   - output summary。
   - output text 片段。
   - 截断状态。
6. 错误关联
   - 如果存在同 request id 的 ops 错误，显示错误摘要并可跳转。

### 导出

沿用现有 Usage 导出，不新建导出系统。

新增导出字段：

- request prompt
- system prompt summary
- tool names
- tool call names
- output summary
- IP location
- request body hash
- truncated flags

导出仍按当前筛选和分页逻辑执行。

## IP 与归属地

当前运行在 Docker compose 中，应用容器映射到本目录：

- compose 文件：`/home/div/1_Project_dir/AI/sub2api/docker-compose.yml`
- 应用端口：宿主机 `8420` -> 容器 `8080`
- 持久化：`data/`、`postgres_data/`、`redis_data/`、`logs/`

IP 获取规则：

1. 优先使用受信任代理链中的真实客户端 IP。
2. 如果没有可信代理配置，保留当前 `ip_address`，同时标记来源。
3. `172.17.0.1` 这类 Docker 网关地址不能伪装成用户公网 IP。
4. 归属地由后台异步解析并缓存，不影响请求主链路。

后台设置里应明确提示：如果要准确显示公网 IP，需要配置 trusted proxy / 转发头信任链。

## 本地测试运行方式

开发和测试必须在 `sub2-fork` 分支。

当前本地服务已按 compose 启动：

```bash
docker compose up -d
docker compose ps
curl http://127.0.0.1:8420/health
```

健康结果应为：

```json
{"status":"ok"}
```

改后端或前端源码后，如容器未热更新，需要重新构建：

```bash
docker compose build sub2api
docker compose up -d
```

不要清理 `.env`、`data/`、`postgres_data/`、`redis_data/`、`logs/`。

## 测试策略

后端测试：

- `ExtractRequestPrompt` 现有用例继续保留。
- 新增 usage audit extractor 单元测试。
- 覆盖 Chat Completions、Responses、Codex instructions、tools、SSE tool call。
- 覆盖截断、hash、敏感字段脱敏。
- 覆盖 usage 成功写库和 ops 错误写库。

前端测试：

- UsageTable 显示新列摘要。
- 预览弹窗分块显示完整内容。
- 缺字段时显示 `-`，不报错。
- 导出包含新增字段。

运行验证：

- 容器启动健康。
- 真实发起一次测试请求后，Usage 页面出现新审计字段。
- 数据库中对应 `usage_logs` 行能查到新增字段。
- 失败请求进入 `ops_error_logs`，并能在错误详情中看到审计字段。

## 非目标

首轮不做：

- 独立请求日志中心。
- 新增 `request_capture_logs` 主表。
- 重做 CPA 页面结构。
- 补全历史请求原文。
- 保存完整未截断原始请求体和响应体。
- 用户端独立客户端。

## 结论

Sub2API 二开应沿用现有 Usage 使用记录体系。成功请求增强 `usage_logs`，失败请求增强 `ops_error_logs`，前端继续在 Usage 页面里通过摘要、预览、导出完成请求审计。
