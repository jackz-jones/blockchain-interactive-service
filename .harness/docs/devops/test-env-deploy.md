# 测试环境部署（ChainWeaver Env）

> 默认直接用 HTTP 调用项目已有的 ChainWeaver Test（CWT）地址，由 CWT 转发 ChainWeaver Env（CWE），部署待测版本、取得测试连接信息，并在测试结束后释放环境。不假设存在 `ida-env` 之类的远端 CLI 或项目自研封装——它们只是同一批接口的调用形式之一，缺失时按本文的命令行直接调用即可。
> 本文档由 `sop.init` 维护；初始化或升级时刷新 `## 项目自定义` 之前的内容，保留下方项目自定义区。

## 1. 部署准备

- 从项目已有配置读取 CWT 地址（组件项目为 `it/config/it.yaml` 的 `cwt.url`），以及 `module` 和 `service`；缺少地址或 `module` 时停止，不默认使用 `ida`。CWT 根据 `modules.<module>.cwe_server` 选择 `cwe_servers.<名称>.url` 中的 CWE 地址，项目无需另配 CWE IP 和端口。将配置来源填入「项目自定义」。
- 确认服务、远端分支和预期 commit。CWE 拉取远端源码，本地未推送的改动不会被部署；提前提交、推送按 `plans/_template/` 中 Commit 阶段的「ChainWeaver 部署所需的提前提交」取得授权。
- 部署请求的超时应覆盖构建、部署及失败恢复窗口；客户端超时不代表服务端已停止执行。

## 2. 申请与部署

### 默认：通过 CWT 部署

直接 HTTP 调用 CWT `POST /api/v1/environment-deployments`。组件项目从 `it/config/it.yaml` 读取 `module` 和 `service`，传入该 `module`、至少包含该 `service` 的 `services` 列表和已推送的 `branch`；需要热更其他服务时加入同一个列表。CWT 读取模块配置并调用 CWE。请求头必须带 8–128 位 `Idempotency-Key`（字母、数字或 `._:-`），为本次部署生成一个稳定编号；同一请求重试复用原编号和参数，部署新版本换新编号。CWT 地址未在项目中配置时，先向环境维护者取得，不猜测地址或用其他端口代替。

```bash
# 申请环境并部署（Idempotency-Key 由调用方生成并复用）
curl -sS -m 3000 -X POST "<CWT_URL>/api/v1/environment-deployments" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: <本次部署的稳定编号>" \
  -d '{"module":"<module>","services":["<service>"],"branch":"<已推送的分支>"}'
```

同一环境重部署前，组件项目核对原环境的应用权限模式与 YAML 的 `module` 一致（`ida` 为 `1`，`dpm` 为 `2`）；不一致则停止。沿用原环境凭证，将其 JSON 原样放进 `X-Environment-Credential` 请求头，只传 services 和 branch；不传 module、database、version、owner，CWT 按凭证定位原 CWE 和 lease。

核对返回的 `env`、`api_url`、`expires_at` 和 services 中的 commit、image，将 `credential` 原样作为 `CHAINWEAVER_ENV` 交给 IT。回执先落到项目已忽略的受限文件（如 `.harness/.env-deploy-receipt.json`，权限 0600），再从文件取值，避免凭证进入 shell 历史、任务文档或提交。

已有租约的连接信息通过 CWT `POST /api/v1/environment-connections` 获取；该接口用 `Authorization: Bearer <lease>` 鉴权，请求体传 `cwe_server`，不使用部署接口的 `X-Environment-Credential` 请求头：

```bash
curl -sS -m 60 -X POST "<CWT_URL>/api/v1/environment-connections" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer <lease>" \
  -d '{"cwe_server":"<cwe_server>"}'
```

部署失败时也保存回执及其中的 `credential`，根据 `lease_usable`、`recovered` 和 `recovery_error` 决定修复、释放或人工处理，不能因请求失败丢弃收尾凭证。

### 补充：项目已有 CWE 直连入口

仅使用项目已有的 CWE 地址配置。直接调用 CWE HTTP 时，请求体使用 JSON，设置 `Content-Type: application/json`；以下接口和回执字段均属于 CWE。

| 操作 | 接口 | 参数与结果 |
|------|------|-----------|
| 申请环境并部署 | `POST /api/v1/deploy` | `services` 数组指定服务，`branch` 指定远端分支；不传 branch 仅占用环境 |
| 同一环境重部署 | `POST /api/v1/deploy` | 增加当前 `lease`，配合 services、branch 部署；不同时提交环境筛选条件、owner 或 ttl，不续租 |
| 读取当前连接信息 | `GET /api/v1/leases/{lease}/environment` | 读取有效 lease 的环境与地址，不申请环境或续租 |

Deploy 必须带 `Idempotency-Key` 请求头（8–128 位字母、数字或 `._:-`）。同一请求的网络重试复用原 key；部署新版本使用新 key。响应丢失时先确认原请求结果，不换 key 盲目重发。

Deploy 同步返回 HTTP 200 后，核对回执中的 `env`、`lease`、`expires_at`、`api_url` 及 `services` 中各服务的 commit、image 和连接信息，确认实际版本与待测版本一致。连接地址按项目协议解析，单个 TCP 端口不能同时当作 HTTP 和 RPC 地址。

使用 CWT 的项目，将部署回执返回的环境凭证通过 `CHAINWEAVER_ENV` 注入测试进程；不要将原始 CWE JSON 当成 CWT 凭证。凭证只保存到项目忽略的受限文件或进程环境，不写入任务文档或仓库。

部署失败时，检查回执中的 `stage`、`published`、`lease_usable`、`recovered`、`recovery_error`，确认是否已变更环境、lease 是否仍可用及恢复结果，再决定修复或重新申请。

## 3. 释放环境

外层取得租约后立即登记释放责任，部署失败、测试失败和进程中断均需处理原租约。测试结束后，由持有环境的外层直接调用 CWT Free（`POST /api/v1/environment-releases`），从原 `credential` JSON 取出 `cwe_server` 和 `lease`：请求体传 `cwe_server`，请求头传 `Authorization: Bearer <lease>`；需要失败取证时传入 `capture_logs: true`。释放接口不使用部署/连接接口的 `X-Environment-Credential` 请求头。CWE 直连项目才调用 `POST /api/v1/free`，请求体必填 `lease`。

```bash
curl -sS -m 180 -X POST "<CWT_URL>/api/v1/environment-releases" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer <lease>" \
  -d '{"cwe_server":"<cwe_server>","capture_logs":false}'
```

- HTTP 202、`accepted: true` 表示服务端已可靠接管恢复，不表示环境已经恢复空闲。全流程验收以原 lease 的恢复成功回执和租约删除判定本次 Free；环境后续的 `idle`、`ready`、`occupied` 状态仍需跟进。随后新操作造成的 `capturing`、`quarantined` 应单独归属，不倒推原 Free 失败。
- 已完成释放的重复请求返回 HTTP 200；核对回执，避免重复释放。
- 需要等待原 lease 恢复终态时（有 CLI 封装或用 CWE 查询接口），按该操作记录：`idle` 为恢复完成，`quarantined` 为恢复失败、需要人工处理；后续独立操作的同名状态不计入本次 Free。
- Commit 阶段核对已有释放回执，区分「已接管」与「已恢复」。

## 4. 失败日志

先保存脱敏后的测试输出和 request ID。CWT 当前未提供远程日志查询接口；归档日志通过原 CWE 的已有运维入口查询，无法取得入口时请环境维护者协助。

Free 的 `capture_logs` 在恢复前采集最近 10 分钟的 current / previous 日志，归档保留 7 天；采集失败不阻断环境恢复。

CWE 提供 `POST /api/v1/logs/search`，提交 `lease` 和 `requestid` 查询归档中的匹配日志；直连时使用原 CWE 地址，不将该路径拼到 CWT 地址上。该接口不实时查询运行中的 Pod：

| 返回 | 含义 |
|------|------|
| 200，text/plain | 匹配日志；`X-CWE-Logs-Status` 区分 complete / partial |
| 202 | 日志仍在采集，按 Retry-After 重试 |
| 404 / 410 | 归档不存在 / 已过期 |

测试断言、业务数据清理及用例结果按集成测试规范执行。

---

## 项目自定义

> 本章节内容**不会**被 skill 刷新覆盖，请按项目实际情况填充。

### 组件清单

| 组件名 | 仓库 / 路径 | 主分支 | 备注 |
|--------|------------|--------|------|
| chain-interactive-service | `git@github.com:jackz-jones/blockchain-interactive-service.git` | `master` | 本仓库；含 web/ 前端（镜像内不含，独立构建） |
| 镜像仓库 | `192.168.1.2:5000` | — | 私有 registry |

### 常用环境

| 环境名 | 用途 | 负责人 | 备注 |
|--------|------|--------|------|
| 本地 dev | `make start-service` | 各自 | Docker 起 PG/Redis（见 development.md） |
| K8s 测试环境 | helm 部署（`deploy/helm/`） | jackz | `make build-docker` 构建镜像后 helm 升级；副本 2 |
| <!-- TODO(sop.init) 如接入 ChainWeaver Test（CWT）环境，在此补充 cwt.url 与 module/service --> | | | |

### 项目特有同步规则 / 排雷点

- **镜像构建含 vendor 化**：`make build-docker` 会 `go mod vendor` + `modvendor` 复制 chainmaker C 依赖，耗时较长；推送双 tag（`:${VERSION}` 与 `-${REV}:${VERSION}`）。
- **测试环境部署路径**：当前不经过 CWT/CWE，直接 `make build-docker` → `helm upgrade`（`deploy/helm/values.yaml` 中 DB 指向 `postgresql:5432`、Redis 指向 `redis:6379` 外部服务）。
- **配置同步**：`etc/` 随镜像 COPY（Dockerfile line 21），改配置需重建镜像或用 ConfigMap 覆盖。
- **链节点依赖**：测试环境需可达 Ethereum 测试网 / ChainMaker 测试链 / Solana devnet；租户链配置经控制台录入并 `test-connection` 预检。
- **优雅退出**：服务先关再释放 SDK（踩坑 `c2afd2c5`），验证部署时注意 shutdown 日志顺序。
