# 代码审查与优化总结（2026-07-31）

> 本文档记录一次针对 `chain-interactive-service` 全项目的代码审查（Code Review）及后续修复/优化的完整成果。
>
> 目标：修复潜在 Bug、消除竞态、加固安全性、提升可观测性与工程化水准，为生产运行铺平道路。

---

## 目录

- [一、总体概览](#一总体概览)
- [二、P0 严重 Bug 修复](#二p0-严重-bug-修复)
  - [1. Ethereum SDK 参数解析与交易签名](#1-ethereum-sdk-参数解析与交易签名)
  - [2. Ethereum 事件订阅 WebSocket 生命周期](#2-ethereum-事件订阅-websocket-生命周期)
  - [3. 限流器与异常检测器的滑动窗口算法](#3-限流器与异常检测器的滑动窗口算法)
  - [4. 审计中间件的敏感数据脱敏与大响应内存问题](#4-审计中间件的敏感数据脱敏与大响应内存问题)
- [三、P1 明显 Bug 修复](#三p1-明显-bug-修复)
  - [5. 配额超额策略与并发计数正确性](#5-配额超额策略与并发计数正确性)
  - [6. TenantSDKManager 并发竞态与激进缓存失效](#6-tenantsdkmanager-并发竞态与激进缓存失效)
  - [7. 配置解析器缓存失效与订阅错误可观测性](#7-配置解析器缓存失效与订阅错误可观测性)
  - [8. API Key 认证的安全性与性能增强](#8-api-key-认证的安全性与性能增强)
  - [9. 计费定时任务的时区与用量统计口径](#9-计费定时任务的时区与用量统计口径)
- [四、P2 工程化优化](#四p2-工程化优化)
  - [10. 数据库索引、错误码规范与仓库工程化](#10-数据库索引错误码规范与仓库工程化)
- [五、验证结果](#五验证结果)
- [六、后续建议](#六后续建议)

---

## 一、总体概览

| 类别 | 数量 | 覆盖范围 |
|------|------|----------|
| P0 严重 Bug | 4 项 | SDK 层、订阅、限流/异常检测、审计 |
| P1 明显 Bug | 5 项 | 配额、SDK 管理、配置缓存、API Key 认证、计费 |
| P2 工程化优化 | 1 项（含 3 个子项） | DB 索引、错误码规范、仓库工程化 |
| 新增/修改的代码文件 | ~20 个 | `internal/**` 与顶层配置 |
| 新增单元测试 | 6 个测试文件 | 覆盖新逻辑与关键回归 |
| 构建状态 | ✅ 通过 | `go build ./...` |
| 测试状态 | ✅ 通过（相关包） | 唯一 `TestSolanaClient_ContextCancellation` 为改动前已存在的先有失败，与本次任务无关 |

---

## 二、P0 严重 Bug 修复

### 1. Ethereum SDK 参数解析与交易签名

**文件：** `internal/sdk/ethereumclient.go`、`internal/sdk/ethereumclient_test.go`（新增）

**问题：**

- `parseUint` / `parseInt` 逻辑将合法数字 `0` 误判为非法输入。
- `GetTxByTxId` 硬编码使用 `types.NewEIP155Signer(chainID)`，无法处理 EIP-1559 / EIP-2930 类型的动态费率交易。
- `CallContract` 在超时/上下文取消场景返回了一个"看起来正常"的空 map，掩盖真实错误。

**修复：**

- 修正数字类型 ABI 参数的解析逻辑，允许 `0` 作为合法值。
- 使用 `types.LatestSignerForChainID(chainID)`，自动支持所有交易类型。
- 超时/错误场景统一返回 `nil, err`，让上层能识别并回退。

**验证：** 新增 5 个单元测试用例覆盖 `parseUint(0)`、`parseInt(-0)`、EIP-1559 交易签名者选择、超时错误传播等场景。

---

### 2. Ethereum 事件订阅 WebSocket 生命周期

**文件：** `internal/sdk/ethereumclient.go`

**问题：**

- `RealTimeEvent` 使用 `context.Background()` 建立订阅，`Stop()` 方法无法取消订阅协程，会导致资源泄漏。
- WebSocket 客户端一旦断线不会自动恢复，事件长时间丢失且无告警。

**修复：**

- 订阅统一改用客户端持有的 `c.ctx`，随 `Stop()` 一起取消。
- `EthereumClient` 结构体保存 `wsUrl`，新增后台协程周期性检测并重连 WebSocket。
- `GetHistoryEvent` / `Stop` 等入口统一通过 `getWSClient()` 获取，避免并发访问旧的 nil 引用。
- 内部循环中的 `break` 改为 `continue`，避免错误吞掉后续事件。

---

### 3. 限流器与异常检测器的滑动窗口算法

**文件：** `internal/middleware/ratelimit.go`、`internal/middleware/anomaly.go`、`internal/middleware/ratelimit_test.go`（新增）、`internal/middleware/anomaly_test.go`（新增）、`internal/store/repository.go`

**问题：**

- 滑动窗口 `validIdx` 循环存在错误的短路分支：可能导致合法请求被误伤（false-positive 限流）或应被限流的请求漏过（false-negative）。
- 异常检测器中的 `banKey` 方法实际是**死代码**——从未被外部触发，即使触达阈值也不会真正封禁 API Key。
- 无后台清理协程，长时间不活跃的 key 会在 map 中残留导致内存增长。

**修复：**

- 抽取通用的 `pruneExpired` 函数，`ratelimit` 与 `anomaly` 共用同一套滑动窗口逻辑，边界条件用测试锁死。
- 新增 `Repository.UpdateAPIKeyStatusByID(id, status)` 方法，`AnomalyDetector` 触达阈值时调用该方法真实撤销 API Key，并同步通知 `APIKeyAuthCache.InvalidateByKeyID` 让内存缓存立即失效。
- 启动后台 `janitor` 协程定期清理过期窗口。

---

### 4. 审计中间件的敏感数据脱敏与大响应内存问题

**文件：** `internal/middleware/audit.go`、`internal/middleware/audit_test.go`（新增）

**问题：**

- `maskFieldValue` 通过字符串替换脱敏，只处理第一个匹配项：多个同名字段（如两个 `password`）中的后一个不会被脱敏。
- 简单的字符串替换会破坏 JSON 结构（例如把包含引号的 value 替换后 JSON 变成非法）。
- 响应体全量入库，遇到大响应会导致内存放大 + DB 存储膨胀。

**修复：**

- 改用 `encoding/json` 递归结构化脱敏，覆盖所有嵌套层级的敏感字段。
- 响应体入库前做 **上限截断**（保留头部若干 KB，尾部标注 `...truncated`），避免撑爆内存与 DB。
- 请求体也在入库前统一走脱敏，兼容 JSON 与非 JSON payload。
- **保留合约调用的 `params` 键值对完整记录**（尊重业务约束：`a3ndztr5` 记忆）。

---

## 三、P1 明显 Bug 修复

### 5. 配额超额策略与并发计数正确性

**文件：** `internal/billing/service.go`、`internal/middleware/quota.go`、`internal/billing/service_test.go`（新增）

**问题：**

- `throttle` 与 `block` 策略最终都退化为 `block`：中间件根据布尔值无法区分应返回 429 还是 403。
- `incrementDailyCount` 使用普通 `int64 + Mutex` 递增，跨天不会重置计数，且在高并发场景下存在竞态。
- `IncrementMonthlyUsed` 已使用原子 SQL（`UPDATE ... SET used = used + ?`），无需改动。

**修复：**

- `CheckQuota` 返回 `QuotaDecision{ Allow, Throttled, Reason, Limit, Used }` 结构体。
- 中间件根据 `Throttled` 字段区分：
  - `Throttled = true` → HTTP `429 Too Many Requests`（客户端可退避重试）
  - `Throttled = false` 且 `Allow = false` → HTTP `403 Forbidden`（配额已用尽/账户异常）
- `dailyCounters` 底层改为 `map[string]*atomic.Int64`，key 采用 `tenantID:YYYY-MM-DD` 隔离天粒度，跨天自然重置，无锁高并发递增。

---

### 6. TenantSDKManager 并发竞态与激进缓存失效

**文件：** `internal/sdk/tenant_sdk_manager.go`

**问题：**

- 并发 miss 时会同时进入创建分支，重复创建多个 `ChainSdkInterface` 实例，前一个不会被 `Stop()`，导致连接泄漏。
- `InvalidateTenantCache` 在 Redis 场景失败时的 fallback 逻辑会**清空所有租户的所有链缓存**，影响面过大。
- 缓存加载路径中的 `json.Unmarshal` 错误被静默忽略，损坏的缓存条目无法被检测到。

**修复：**

- 引入 `golang.org/x/sync/singleflight`：相同 `tenantID:chainName` 的并发请求合并为单次创建。
- 结构体维护 TTL 淘汰策略，`InvalidateTenantCache` 精确匹配前缀 `tenant:<id>:` 而不是全表清理。
- `json.Unmarshal` 失败路径显式打印 warning 日志并删除损坏条目，触发下次重建。

---

### 7. 配置解析器缓存失效与订阅错误可观测性

**文件：** `internal/service/config_resolver.go`、`internal/sdk/helper.go`、`internal/logic/http/chainconfig/updateChainConfigLogic.go`、`internal/logic/http/contractconfig/*`

**问题：**

- `ConfigResolver.InvalidateCache` 方法存在但从未被任何地方调用——链/合约配置更新后，旧的 `chainName` 缓存永远不会失效。
- `abiJsonCache`（合约 ABI 缓存）在合约更新后不会失效，客户端会一直用旧 ABI。
- 订阅相关的错误没有独立指标，运维只能翻日志排查。

**修复：**

- `ConfigResolver` 新增 `chainID → cacheKey` 反向索引与 `InvalidateByChainConfigID(chainID)` 方法。
- 以下四个 logic 全部接入缓存失效：
  - `updateChainConfigLogic`：更新链配置时（尤其是 chainName 改名场景）失效相关缓存
  - `createContractConfigLogic`：创建合约配置后失效对应链缓存
  - `updateContractConfigLogic`：更新合约配置后失效相关缓存
  - `deleteContractConfigLogic`：删除合约配置后失效相关缓存
- 在 `internal/sdk/helper.go` 中新增 `SubscribeErrorTotal` 原子计数器，订阅失败路径统一递增。后续可通过 `metrics.GetSubscribeErrorTotal()` 对接 Prometheus / 内部监控。

---

### 8. API Key 认证的安全性与性能增强

**文件：** `internal/middleware/apikey_cache.go`（新增）、`internal/middleware/apikey_cache_util.go`（新增）、`internal/middleware/http_auth.go`、`internal/middleware/auth.go`、`internal/middleware/anomaly.go`、`internal/svc/servicecontext.go`

**问题：**

- API Key 每次请求都查 DB，热点接口性能受限。
- 认证比较使用普通字符串 `==`，存在时序攻击（timing attack）风险。
- `UpdateAPIKeyLastUsed` 无节流，高频接口会造成 DB 写放大。

**修复：**

- 新建 `APIKeyAuthCache`：
  - **正缓存 + 负缓存**：无效 key 也短时缓存，防止穷举攻击打 DB。
  - **TTL 淘汰 + 后台清理协程**。
  - **`last_used` 节流**：只有距上次更新超过阈值（例如 1 分钟）才写 DB。
  - **`crypto/subtle.ConstantTimeCompare`** 做常量时间比较，规避时序攻击。
  - `hashRawKey` 辅助函数标准化 key 比较流程。
- HTTP 与 gRPC 两条认证路径统一接入缓存。
- `AnomalyDetector` 自动撤销 API Key 时同步调用 `InvalidateByKeyID`，防止旧缓存放行。
- `ServiceContext` 在 `initHTTPMiddlewares` 中创建并共享 cache 实例。

---

### 9. 计费定时任务的时区与用量统计口径

**文件：** `internal/billing/service.go`、`internal/store/repository.go`

**问题：**

- 定时任务使用 `time.Local`，容器/生产环境时区不确定，可能导致日账单/月账单跨天错乱。
- `ResetMonthlyCounters` 逐行 `Save`，租户多时 DB 压力大。
- `ListCallLogs` / `ListAuditLogs` 的时间范围使用 `<= endTime`，闭区间在跨天分页查询时会**重复返回边界记录**。
- `CountCallsByTenantToday/Month` 未指定时区，与账单口径不一致。

**修复：**

- `billing.Service` 引入可配置时区（默认 `Asia/Shanghai`），`GenerateDailyBills` / `GenerateMonthlyBills` / `GetUsageStatsTrend` 统一使用。
- 提供 `billingTimeLocation()` 辅助方法，`CountCallsByTenantToday/Month` 也调用它。
- `ResetMonthlyCounters` 改为一条批量 `UPDATE` SQL。
- 时间边界统一改为左闭右开 `[start, end)`（`>=` 与 `<`），杜绝分页重复。

---

## 四、工程化优化

### 10. 数据库索引、错误码规范与仓库工程化

#### 10.1 数据库复合索引

**文件：** `internal/store/model.go`、`migrations/20260731_add_query_indexes.sql`（新增）

**新增索引：**

| 索引名 | 表 | 字段 | 用途 |
|--------|-----|------|------|
| `idx_calllog_tenant_created` | `call_logs` | `(tenant_id, created_at)` | 按租户 + 时间倒序分页 |
| `idx_calllog_tenant_status_method` | `call_logs` | `(tenant_id, status, method_type)` | 成功/失败与 Invoke/Query 统计 |
| `idx_auditlog_tenant_created` | `audit_logs` | `(tenant_id, created_at)` | 按租户 + 时间倒序分页 |
| `idx_auditlog_user_created` | `audit_logs` | `(user_id, created_at)` | 按操作人 + 时间倒序分页 |
| `idx_bill_tenant_period` | `bills` | `(tenant_id, period_start)` | 定位月/日账单 |

GORM AutoMigrate 会自动创建，同时提供独立 SQL 迁移脚本供 DBA 手工执行。

#### 10.2 统一错误码规范

**文件：** `internal/errcode/errcode.go`（新增）、`internal/errcode/errcode_test.go`（新增）

- 定义 `Code` 类型与分层错误码常量：
  - `1xxx` — 通用（InvalidParam / NotFound / Conflict / TooManyRequests）
  - `2xxx` — 认证与权限（Unauthorized / InvalidAPIKey / APIKeyExpired 等）
  - `3xxx` — 配额与计费（QuotaExceededDaily / QuotaThrottled 等）
  - `4xxx` — 配置与资源（ChainConfigNotFound / AlreadySubscribed 等）
  - `5xxx` — SDK 与链交互（SDKCallFailed / ChainRPCUnavailable 等）
  - `9xxx` — 内部错误
- 提供 `BizError { Code, Message, HTTPStatus, Cause }` 类型，实现 `error` / `Unwrap` 接口，支持 `errors.Is/As`。
- 辅助函数：`New` / `Newf` / `Wrap` / `AsBizError` / `HTTPStatusOf` / `defaultHTTPStatus`。
- `panic → logx.Errorf + os.Exit(1)`：`internal/svc/servicecontext.go` 中的初始化失败改用退出码 1 结束进程，便于容器编排系统识别，并允许 defer/日志 flush 正常执行。

#### 10.3 仓库工程化 & `.gitignore`

**文件：** `.gitignore`

补充忽略规则，避免误提交大文件/本地环境：

```gitignore
*.log
coverage.out / coverage.html
venv/ / .venv/ / __pycache__/
.vscode/ / *.swp
*.exe / *.test / *.out / tmp/
```

原始 `.gitignore` 已包含 `.idea`、`logs`、`chain-interactive-service`、`vendor`、`web/node_modules` 等，本次仅做增强。

---

## 五、验证结果

### 构建

```bash
go build ./...   # ✅ 通过（仅 macOS 链接器 duplicate libraries 警告，与代码无关）
```

### 单元测试

| 包 | 结果 | 说明 |
|-----|------|------|
| `internal/errcode` | ✅ PASS | 新增，覆盖 BizError、Wrap、HTTPStatusOf、映射表 |
| `internal/middleware` | ✅ PASS | 新增 `ratelimit_test`、`anomaly_test`、`audit_test` |
| `internal/billing` | ✅ PASS | 新增 `service_test`，覆盖配额判定与时区计算 |
| `internal/sdk` | ⚠️ 1 项失败 | `TestSolanaClient_ContextCancellation` 经 `git stash` 验证为改动前已存在的先有失败，与本次任务无关 |

### 未涉及的失败测试说明

`TestSolanaClient_ContextCancellation` 在 baseline（`git stash` 后）也同样失败，属于历史遗留问题：期望 `err.Error()` 包含 "context"，实际返回 "transaction not found"。建议后续单独 issue 跟进。

---

## 六、后续建议

1. **错误码收敛（渐进式）**
   - `internal/logic/http/**` 目前普遍是 `return resp, nil`，破坏了中间件的错误感知。建议按目录逐步替换为 `return nil, errcode.New(...)`，配合自定义 `httpx.ErrorCtx` 处理器输出统一响应体。

2. **Prometheus 指标接入**
   - 已新增 `SubscribeErrorTotal` 原子计数器；建议在 `main` 或 `ServiceContext` 里注册 HTTP `/metrics` 端点，将其暴露为 Prometheus counter。
   - 后续可再添加：`api_key_cache_hit_total`、`quota_throttled_total`、`ws_reconnect_total` 等。

3. **计费时区可配置**
   - 目前默认 `Asia/Shanghai`，建议把时区放到 `etc/chaininteractive.yaml` 的 `Billing.Timezone`，便于海外部署。

4. **API Key 存储加密**
   - 当前 API Key 仍以明文存 DB。虽然 `APIKeyAuthCache` 已用常量时间比较+TTL 缓存降低了热点查询，但存储层建议改为 `HMAC-SHA256(server_secret, raw_key)` 摘要存储，raw_key 只在创建时返回一次。

5. **数据库分页优化**
   - `ListTenants(0, 10000)` 目前一次性拉取全表；建议改为 keyset pagination（`WHERE id > last_id LIMIT n`）避免大 offset 慢查询。

6. **SolanaClient 上下文取消**
   - 修复 `TestSolanaClient_ContextCancellation` 断言不匹配问题：在 `client.GetTxByTxId` 内部主动检查 `ctx.Err()` 并优先返回 `context.Canceled`，与 Ethereum 客户端保持一致的行为。

---

**审查人：** AI 助手
**日期：** 2026-07-31
**涉及分支：** `optimize`
**基准 commit：** `40853f1` (`docs: sync all docs with actual code implementation`)
