# Code Review & Optimization Summary (2026-07-31)

> This document records a full-project code review of `chain-interactive-service` and the subsequent bug fixes / optimizations.
>
> Goal: fix latent bugs, eliminate race conditions, harden security, improve observability and engineering quality — paving the way for production.

---

## Table of Contents

- [1. Overview](#1-overview)
- [2. P0 Critical Bug Fixes](#2-p0-critical-bug-fixes)
  - [2.1 Ethereum SDK: Param Parsing & Tx Signer](#21-ethereum-sdk-param-parsing--tx-signer)
  - [2.2 Ethereum Event Subscription WebSocket Lifecycle](#22-ethereum-event-subscription-websocket-lifecycle)
  - [2.3 Sliding-Window Algorithm in Rate Limiter & Anomaly Detector](#23-sliding-window-algorithm-in-rate-limiter--anomaly-detector)
  - [2.4 Audit Middleware: PII Masking & Large-Response Memory](#24-audit-middleware-pii-masking--large-response-memory)
- [3. P1 Bug Fixes](#3-p1-bug-fixes)
  - [3.1 Quota Over-Limit Strategy & Concurrent Counting](#31-quota-over-limit-strategy--concurrent-counting)
  - [3.2 TenantSDKManager Race Condition & Aggressive Cache Invalidation](#32-tenantsdkmanager-race-condition--aggressive-cache-invalidation)
  - [3.3 ConfigResolver Cache Invalidation & Subscription Metrics](#33-configresolver-cache-invalidation--subscription-metrics)
  - [3.4 API Key Auth: Security & Performance Enhancement](#34-api-key-auth-security--performance-enhancement)
  - [3.5 Billing Cron: Timezone & Usage-Stats Boundary](#35-billing-cron-timezone--usage-stats-boundary)
- [4. P2 Engineering Improvements](#4-p2-engineering-improvements)
  - [4.1 DB Indexes, Error-Code Convention & Repo Hygiene](#41-db-indexes-error-code-convention--repo-hygiene)
- [5. Verification](#5-verification)
- [6. Follow-Up Recommendations](#6-follow-up-recommendations)

---

## 1. Overview

| Category | Count | Scope |
|----------|-------|-------|
| P0 Critical Bugs | 4 | SDK layer, subscriptions, rate limit / anomaly, audit |
| P1 Bugs | 5 | Quota, SDK manager, config cache, API-Key auth, billing |
| P2 Engineering | 1 (3 sub-items) | DB indexes, error codes, repo hygiene |
| Files added / modified | ~20 | `internal/**` and top-level config |
| Unit tests added | 6 files | Covering new logic and key regressions |
| Build | ✅ Pass | `go build ./...` |
| Tests | ✅ Pass (touched packages) | The only failing test `TestSolanaClient_ContextCancellation` is a pre-existing failure unrelated to this work (confirmed via `git stash`) |

---

## 2. P0 Critical Bug Fixes

### 2.1 Ethereum SDK: Param Parsing & Tx Signer

**Files:** `internal/sdk/ethereumclient.go`, `internal/sdk/ethereumclient_test.go` (new)

**Problems:**

- `parseUint` / `parseInt` treated the legitimate value `0` as invalid input.
- `GetTxByTxId` hard-coded `types.NewEIP155Signer(chainID)` and could not decode EIP-1559 / EIP-2930 dynamic-fee transactions.
- `CallContract` returned an "innocent-looking" empty map on timeout / ctx-cancel, masking the real error.

**Fixes:**

- Corrected numeric ABI parsing to allow `0` as a valid value.
- Switched to `types.LatestSignerForChainID(chainID)` which handles every transaction type.
- Timeout / error paths now return `nil, err` so callers can detect and fall back.

**Verification:** Added 5 unit-test cases covering `parseUint(0)`, `parseInt(-0)`, EIP-1559 signer selection, timeout error propagation, etc.

---

### 2.2 Ethereum Event Subscription WebSocket Lifecycle

**File:** `internal/sdk/ethereumclient.go`

**Problems:**

- `RealTimeEvent` used `context.Background()`, so `Stop()` could not cancel the subscription goroutine — resource leak.
- The WebSocket client never reconnected once dropped, silently losing events with no alarm.

**Fixes:**

- Subscriptions now use `c.ctx` (client-owned context) — cancelled by `Stop()`.
- The struct stores the `wsUrl`; a background goroutine periodically probes and reconnects the WS client.
- `GetHistoryEvent` / `Stop` fetch the current client via `getWSClient()` to avoid racing with a stale nil.
- Inner-loop `break` corrected to `continue` so a single decode error no longer aborts the whole subscription.

---

### 2.3 Sliding-Window Algorithm in Rate Limiter & Anomaly Detector

**Files:** `internal/middleware/ratelimit.go`, `internal/middleware/anomaly.go`, `internal/middleware/ratelimit_test.go` (new), `internal/middleware/anomaly_test.go` (new), `internal/store/repository.go`

**Problems:**

- The `validIdx` loop had an off-by-one short-circuit branch that could either drop legitimate requests (false-positive throttle) or let excess ones through (false-negative).
- `banKey` in `AnomalyDetector` was **dead code** — never invoked, so hitting the threshold did not actually revoke an API key.
- No background janitor: idle keys accumulated in maps forever, leaking memory.

**Fixes:**

- Extracted a shared `pruneExpired` helper; both `ratelimit` and `anomaly` use it, with boundary conditions locked down by tests.
- Added `Repository.UpdateAPIKeyStatusByID(id, status)`. When threshold is reached, `AnomalyDetector` now actually revokes the key and calls `APIKeyAuthCache.InvalidateByKeyID` so in-memory caches expire immediately.
- Launched a background janitor goroutine that periodically prunes expired windows.

---

### 2.4 Audit Middleware: PII Masking & Large-Response Memory

**Files:** `internal/middleware/audit.go`, `internal/middleware/audit_test.go` (new)

**Problems:**

- `maskFieldValue` masked via string replace and only the **first** match — subsequent occurrences of the same sensitive field (e.g. two `password` values) leaked through.
- Naive string replacement could break JSON structure when the value contained quotes.
- The full response body was persisted to DB — large responses caused memory amplification and DB bloat.

**Fixes:**

- Switched to `encoding/json` recursive structural masking, covering nested levels.
- Response body is now **truncated** to a size cap before persistence (head kept, tail marked `...truncated`).
- Request body is also masked centrally before persistence, supporting both JSON and non-JSON payloads.
- **Preserves complete `params` key-value pairs for contract calls** (business constraint, memory `a3ndztr5`).

---

## 3. P1 Bug Fixes

### 3.1 Quota Over-Limit Strategy & Concurrent Counting

**Files:** `internal/billing/service.go`, `internal/middleware/quota.go`, `internal/billing/service_test.go` (new)

**Problems:**

- `throttle` and `block` strategies collapsed to the same behaviour — the middleware could not distinguish 429 from 403.
- `incrementDailyCount` used `int64` + `Mutex` and never reset across days; also raced under high concurrency.
- `IncrementMonthlyUsed` already used atomic SQL (`UPDATE ... SET used = used + ?`) — no change required.

**Fixes:**

- `CheckQuota` now returns `QuotaDecision{ Allow, Throttled, Reason, Limit, Used }`.
- Middleware maps the decision as:
  - `Throttled = true` → HTTP `429 Too Many Requests` (client should back off)
  - `Throttled = false && Allow = false` → HTTP `403 Forbidden` (quota exhausted / account issue)
- `dailyCounters` re-designed as `map[string]*atomic.Int64`, keyed by `tenantID:YYYY-MM-DD` — natural per-day reset, lock-free increment.

---

### 3.2 TenantSDKManager Race Condition & Aggressive Cache Invalidation

**File:** `internal/sdk/tenant_sdk_manager.go`

**Problems:**

- Concurrent misses could create multiple `ChainSdkInterface` instances; the earlier one was never `Stop()`ed → connection leak.
- `InvalidateTenantCache` fallback on Redis failure wiped **all tenants' all chain caches** — blast radius far too large.
- `json.Unmarshal` errors during cache load were silently swallowed, so corrupt entries stayed forever.

**Fixes:**

- Introduced `golang.org/x/sync/singleflight`: concurrent requests for the same `tenantID:chainName` collapse into a single build.
- Added TTL eviction; `InvalidateTenantCache` now matches the exact `tenant:<id>:` prefix instead of wiping everything.
- `json.Unmarshal` failures now log a warning and delete the corrupt entry so the next hit rebuilds it.

---

### 3.3 ConfigResolver Cache Invalidation & Subscription Metrics

**Files:** `internal/service/config_resolver.go`, `internal/sdk/helper.go`, `internal/logic/http/chainconfig/updateChainConfigLogic.go`, `internal/logic/http/contractconfig/*`

**Problems:**

- `ConfigResolver.InvalidateCache` existed but was **never called** — stale `chainName` cache entries lived forever after config updates.
- `abiJsonCache` (contract ABI cache) never expired after contract updates — clients kept using the old ABI.
- No dedicated metric for subscription failures — ops had to grep logs.

**Fixes:**

- `ConfigResolver` gained a reverse index (`chainID → cacheKey`) and a new `InvalidateByChainConfigID(chainID)` method.
- All four logic paths now invalidate caches:
  - `updateChainConfigLogic` — especially for chainName rename cases
  - `createContractConfigLogic` — invalidate chain cache after contract creation
  - `updateContractConfigLogic` — invalidate after contract updates
  - `deleteContractConfigLogic` — invalidate after contract deletion
- `internal/sdk/helper.go` exposes an atomic `SubscribeErrorTotal` counter incremented on every subscription failure. Can be surfaced to Prometheus / internal monitoring via `metrics.GetSubscribeErrorTotal()`.

---

### 3.4 API Key Auth: Security & Performance Enhancement

**Files:** `internal/middleware/apikey_cache.go` (new), `internal/middleware/apikey_cache_util.go` (new), `internal/middleware/http_auth.go`, `internal/middleware/auth.go`, `internal/middleware/anomaly.go`, `internal/svc/servicecontext.go`

**Problems:**

- Every request hit the DB — hot endpoints throttled by DB latency.
- Plain `==` string comparison exposed a timing-attack vector.
- `UpdateAPIKeyLastUsed` had no throttling — write amplification on hot keys.

**Fixes:**

- New `APIKeyAuthCache`:
  - **Positive + negative caching** — invalid keys are also cached briefly to blunt brute-force scans against the DB.
  - **TTL eviction + background janitor.**
  - **`last_used` throttle** — DB write only if last update was ≥ threshold (e.g. 1 min) ago.
  - **`crypto/subtle.ConstantTimeCompare`** for constant-time equality — no timing side-channel.
  - `hashRawKey` helper standardises the comparison pipeline.
- HTTP and gRPC auth paths share the cache.
- When `AnomalyDetector` auto-revokes a key, it also calls `InvalidateByKeyID` so no stale cache lets it through.
- `ServiceContext` creates and shares the cache instance from `initHTTPMiddlewares`.

---

### 3.5 Billing Cron: Timezone & Usage-Stats Boundary

**Files:** `internal/billing/service.go`, `internal/store/repository.go`

**Problems:**

- Cron jobs used `time.Local` — containers/prod may have unpredictable timezones, drifting daily/monthly bills across day boundaries.
- `ResetMonthlyCounters` `Save`d row-by-row — high DB load for large tenant sets.
- `ListCallLogs` / `ListAuditLogs` used `<= endTime` — the closed interval caused **duplicate records at the boundary** in paginated queries.
- `CountCallsByTenantToday/Month` did not pin a timezone, drifting from the billing convention.

**Fixes:**

- `billing.Service` accepts a configurable timezone (default `Asia/Shanghai`). `GenerateDailyBills` / `GenerateMonthlyBills` / `GetUsageStatsTrend` all use it uniformly.
- Added a `billingTimeLocation()` helper; `CountCallsByTenantToday/Month` reuse it.
- `ResetMonthlyCounters` collapsed into a single batch `UPDATE`.
- Time boundaries unified to half-open `[start, end)` (`>=` and `<`) — pagination duplicates gone.

---

## 4. P2 Engineering Improvements

### 4.1 DB Indexes, Error-Code Convention & Repo Hygiene

#### 4.1.1 Composite DB Indexes

**Files:** `internal/store/model.go`, `migrations/20260731_add_query_indexes.sql` (new)

**New indexes:**

| Index name | Table | Columns | Purpose |
|-----------|-------|---------|---------|
| `idx_calllog_tenant_created` | `call_logs` | `(tenant_id, created_at)` | Tenant + time-desc pagination |
| `idx_calllog_tenant_status_method` | `call_logs` | `(tenant_id, status, method_type)` | Success/failure and Invoke/Query stats |
| `idx_auditlog_tenant_created` | `audit_logs` | `(tenant_id, created_at)` | Tenant + time-desc pagination |
| `idx_auditlog_user_created` | `audit_logs` | `(user_id, created_at)` | Operator + time-desc pagination |
| `idx_bill_tenant_period` | `bills` | `(tenant_id, period_start)` | Locate monthly/daily bills |

GORM AutoMigrate creates them automatically; a standalone SQL migration is also provided for manual DBA execution.

#### 4.1.2 Unified Error-Code Convention

**Files:** `internal/errcode/errcode.go` (new), `internal/errcode/errcode_test.go` (new)

- Defines a `Code` type and layered error-code constants:
  - `1xxx` — Generic (InvalidParam / NotFound / Conflict / TooManyRequests)
  - `2xxx` — Auth & permission (Unauthorized / InvalidAPIKey / APIKeyExpired …)
  - `3xxx` — Quota & billing (QuotaExceededDaily / QuotaThrottled …)
  - `4xxx` — Config & resources (ChainConfigNotFound / AlreadySubscribed …)
  - `5xxx` — SDK & chain interaction (SDKCallFailed / ChainRPCUnavailable …)
  - `9xxx` — Internal
- `BizError { Code, Message, HTTPStatus, Cause }` implements `error` / `Unwrap`, supporting `errors.Is/As`.
- Helpers: `New` / `Newf` / `Wrap` / `AsBizError` / `HTTPStatusOf` / `defaultHTTPStatus`.
- `panic → logx.Errorf + os.Exit(1)`: init failures in `internal/svc/servicecontext.go` exit with code 1, letting container orchestrators detect it and giving defers / log flushes a chance to run.

#### 4.1.3 Repo Hygiene & `.gitignore`

**File:** `.gitignore`

Added rules to prevent large / local-env files from being committed:

```gitignore
*.log
coverage.out / coverage.html
venv/ / .venv/ / __pycache__/
.vscode/ / *.swp
*.exe / *.test / *.out / tmp/
```

The existing `.gitignore` already covered `.idea`, `logs`, `chain-interactive-service`, `vendor`, `web/node_modules`, etc. — this is a superset.

---

## 5. Verification

### Build

```bash
go build ./...   # ✅ passes (only a benign macOS linker "duplicate libraries" warning)
```

### Unit Tests

| Package | Result | Notes |
|---------|--------|-------|
| `internal/errcode` | ✅ PASS | New; covers BizError, Wrap, HTTPStatusOf, mappings |
| `internal/middleware` | ✅ PASS | New tests: `ratelimit_test`, `anomaly_test`, `audit_test` |
| `internal/billing` | ✅ PASS | New `service_test` covering quota decisions & timezone math |
| `internal/sdk` | ⚠️ 1 pre-existing failure | `TestSolanaClient_ContextCancellation` failed on baseline too (verified via `git stash`) — unrelated to this work |

### Note on the failing test

`TestSolanaClient_ContextCancellation` fails on the baseline as well: it asserts `err.Error()` contains `"context"`, but the actual message is `"transaction not found"`. Suggest filing a separate issue.

---

## 6. Follow-Up Recommendations

1. **Gradually roll out error-code adoption**
   - `internal/logic/http/**` still commonly does `return resp, nil`, breaking middleware's error visibility. Recommend migrating directory-by-directory to `return nil, errcode.New(...)` and pairing with a custom `httpx.ErrorCtx` handler for a unified response envelope.

2. **Wire up Prometheus**
   - `SubscribeErrorTotal` is now an atomic counter; register `/metrics` in `main` / `ServiceContext` and expose it as a Prometheus counter.
   - Consider adding `api_key_cache_hit_total`, `quota_throttled_total`, `ws_reconnect_total`, etc.

3. **Configurable billing timezone**
   - Default is `Asia/Shanghai`; expose `Billing.Timezone` in `etc/chaininteractive.yaml` for overseas deployments.

4. **Hash-based API-Key storage**
   - Keys are still stored in plaintext. `APIKeyAuthCache` already mitigates hot-path DB pressure with constant-time compare and TTL, but the storage layer should switch to `HMAC-SHA256(server_secret, raw_key)`; return `raw_key` only once at creation.

5. **Keyset pagination**
   - `ListTenants(0, 10000)` slurps the full table. Move to keyset pagination (`WHERE id > last_id LIMIT n`) to avoid slow large-offset queries.

6. **SolanaClient context-cancel behaviour**
   - Fix `TestSolanaClient_ContextCancellation` by making `GetTxByTxId` check `ctx.Err()` up-front and returning `context.Canceled` first — aligning with the Ethereum client.

---

**Reviewer:** AI assistant
**Date:** 2026-07-31
**Branch:** `optimize`
**Baseline commit:** `40853f1` (`docs: sync all docs with actual code implementation`)
