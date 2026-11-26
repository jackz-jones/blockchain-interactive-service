# 接口文档总索引

> 本项目为 proto + .api 双契约：gRPC 契约在 `proto/chaininteractive.proto`，HTTP 契约在 `api/chaininteractive.api`。
> 新增 / 修改接口时同步更新本索引。**HTTP 路由真源为 `internal/handler/routes.go`**（已手工改造，与 .api 存在已知漂移，见架构文档技术债）。

---

## 命名约定

- proto 契约：列出仓库内实际 `.proto` 路径与 service，不逐接口生成 Markdown。
- HTTP 路由：按 `@server` group 分组索引 `internal/handler/routes.go` 注册的路径。

---

## 模块清单

| 模块 | 路径 | 接口数 |
|------|------|-------|
| gRPC 契约 | `proto/chaininteractive.proto`（生成 `pb/`，service `ChainInteractive`，包名 `proto`） | 4 rpc（1 个 server streaming） |
| HTTP 契约 | `api/chaininteractive.api`（service `chaininteractive-api`，prefix `/api/v1`） | 41 条（以 routes.go 注册为准） |

---

## 接口清单

### gRPC（service `ChainInteractive`，`proto/chaininteractive.proto`）

| rpc | 模块 | 类型 | 契约 / 路由路径 |
|------|------|------|------|
| `GetTxByTxId` | 链交互 | Unary | `proto/chaininteractive.proto:10` |
| `CallContract` | 链交互 | Unary | `proto/chaininteractive.proto:13` |
| `GetAvailableChainAndContractNames` | 链交互 | Unary | `proto/chaininteractive.proto:16` |
| `SubscribeContractEvents` | 事件订阅 | Server Streaming | `proto/chaininteractive.proto:19` |

相关 message：`TxData` / `KeyValuePair` / `ContractDesc` / `ChainAndContractName`；enum：`MethodType`（Invoke/Query）、`ChainType`（Ethereum/Chainmaker/Solana）。

### HTTP（`/api/v1` 前缀，`internal/handler/routes.go`）

| 分组 | 路由数 | 中间件 | 代表路由 |
|------|-------|--------|---------|
| 公开路由 | 2 | 无 | `POST /auth/register`、`POST /auth/validate` |
| 链上操作（计配额） | 2 | Auth+Audit+RateLimit+Quota | `POST /contract/call`、`POST /events/subscribe-by-contract` |
| 链查询 | 4 | Auth+Audit+RateLimit | `GET /tx/:txId`、`GET /chains` 等 |
| 事件 | 6 | Auth+Audit+RateLimit | `GET /events/subscriptions`、`PUT/DELETE /events/subscribe-by-contract/:id` 等 |
| 链配置 | 6 | Auth+Audit+RateLimit | `/chain-configs` CRUD + `test-connection` |
| 合约配置 | 5 | Auth+Audit+RateLimit | `/chain-configs/:id/contracts` CRUD |
| 租户 | 5 | Auth+Audit+RateLimit | `/tenants` CRUD + disable/enable |
| API Key | 2 | Auth+Audit+RateLimit | `/api-keys` |
| 用户 | 1 | Auth+Audit+RateLimit | `GET /users` |
| Dashboard | 8 | Auth+Audit+RateLimit | `/dashboard/overview`、`call-logs`、`bills`、`audit-logs` 等 |

> 已知漂移：`.api` 中的 `EventSubscribe/EventPoll/EventUnsubscribe`（按 subscriptionId 轮询式）未在 routes.go 注册；`SubscribeByContract` 三条已在 routes.go 注册但未在 `.api` 声明。文档以 routes.go 为准。
