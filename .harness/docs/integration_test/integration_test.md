# 集成测试规范（环境 / 用例 / 运行调试）

> 让任意成员（或 AI Agent）能在**真实或近真实环境**把集成测试**搭起来、写得对、跑得通、出错查得到**。
> 三段式结构：① 环境与依赖 / ② 用例设计与组织 / ③ 运行与调试。
>
> **与单元测试的边界**：
> - 单元测试（`.harness/docs/unittest/unittest.md`）：函数 / 模块级，**禁止**真实外部调用，全部 Mock。
> - 集成测试（本文）：跨模块 / 跨服务 / 跨进程链路，**允许**调用真实测试环境的依赖（DB / MQ / 上下游服务 / 第三方 SDK 测试桩）。
>
> **若仓库存在项目集成测试规则文件**（`.codebuddy/rules/integration_test_*.md`、`.cursor/rules/`、`.windsurf/rules/` 等或同义命名如 `it_rule.md`、`api_test_rule.md`，见 `AGENTS.md` 文末「项目既有规则」），那是权威来源：本文以摘录 + 链接形式承接，不另起冲突；调整本文必须同步 rules。

> Source: Makefile（无独立 it target）+ 现有 12 个 UT 文件（无 `*_integration_test.go`）+ `deploy/helm/` + `etc/chaininteractive.yaml`
> Last-verified: 2026-11-23

---

## 一、环境与依赖

### 1. 集成测试形态判定

> 不同项目的"集成测试"含义不同，先明确本项目落在哪一种（可多选），再决定如何搭环境。

| 形态 | 典型场景 | 依赖准备方式 |
|------|---------|-------------|
| **接口级 IT**（API 黑盒）| 调用 HTTP/RPC 接口验证业务流程 | 需要可用的测试环境地址 + 鉴权 |
| **链路级 IT**（多服务联动）| 验证 A→B→C 的端到端链路 | 全链路测试环境 + 数据隔离方案 |
| **数据库 IT** | 验证 DAO / migration / 事务行为 | 独立测试库（容器或共享库 + 隔离 schema） |
| **消息 / 异步 IT** | 验证 MQ 消费 / 定时任务 / 回调 | 独立 topic / queue + 消费组隔离 |
| **前端 IT**（E2E） | 验证页面交互、跨页流程 | Playwright / Cypress + 后端 mock 或测试环境 |

> 在下方填写**本项目实际形态**，并标注权重（哪些为主、哪些为辅）：
>
> **本项目以 接口级 IT 为主（gRPC + HTTP 双入口黑盒验证真实链路：合约调用→链节点→事件订阅→Redis Stream→gRPC 流推送）、数据库 IT 为辅（AutoMigrate 建表与 Repository 行为）；前端无 E2E。**
> 现状：仓库尚无 `*_integration_test.go`，SOP 04 IT 阶段按本规范新增，build tag 统一用 `integration`。

### 2. 测试环境信息

> **强依赖** `.harness/docs/devops/test-env-deploy.md`：环境的"如何创建/部署/同步"在 devops 文档中，本节只记录"集成测试需要的环境形态与连通信息"。

| 项 | 说明 | 示例 / 实际值 |
|----|------|--------------|
| 环境名称 / 类型 | K8s 测试环境（helm 部署，副本 2） | `deploy/helm/values.yaml` |
| 接入地址 | HTTP Gateway `:8080` / gRPC `:8085` / DevServer 探活 `:6061` | 集群内 Service 或 port-forward |
| 鉴权方式 | 租户 API Key（注册 `/api/v1/auth/register` 获取） | 专用测试租户，禁止生产 Key |
| 数据隔离策略 | 独立测试租户 + 租户级链/合约配置（tenant 维度天然隔离） | 测试租户 ID 前缀 `it-` |
| 清理策略 | 测试租户及其链/合约配置用例 teardown 删除 | 手动兜底：dashboard 查 CallLog |
| 上下游依赖 | 见 `.harness/docs/relationship.md`：PG / Redis / 测试网链节点（Ethereum 测试网、ChainMaker 测试链、Solana devnet） | IT 必须全部可达 |

### 3. 前置依赖检查清单（先查再装）

> 任何一项已满足就跳过对应安装步骤，不要无脑重置环境。

```bash
# (1) 测试环境可达性
curl -sS --max-time 5 http://<test-host>:6061/healthz && echo "env OK" || echo "env unreachable"

# (2) 鉴权凭证就绪（不打印明文）
test -n "$CIS_IT_APIKEY" && echo "apikey OK" || echo "缺少 API Key"

# (3) 集成测试依赖就绪
curl -sS --max-time 5 http://<test-host>:8080/api/v1/healthz || kubectl get pods -l app=chain-interactive-service
kubectl get svc postgresql redis 2>/dev/null && echo "deps OK" || echo "缺依赖"

# (4) 测试数据 fixture / 种子数据
# 测试租户/链配置经 POST /api/v1/auth/register + POST /api/v1/tenants 动态创建，无需 SQL fixture

# (5) 隔离资源（独立 topic / namespace 等）
# Redis Stream 按租户+合约维度隔离；订阅用例结束后 DELETE /api/v1/events/subscribe-by-contract/:id
```

判断逻辑：5 项全过 → 直接执行 IT；某项失败 → **只修复失败项**，不要全量重建。

### 4. 环境变量与配置

| 变量 | 用途 | 示例 |
|------|------|------|
| `CIS_IT_APIKEY` | 测试租户 API Key（**严禁硬编码**） | 从注册接口签发 / 本地 `.env.local`（已 gitignore）注入 |
| `CIS_IT_HTTP` | HTTP Gateway 地址 | `http://<test-host>:8080` |
| `CIS_IT_GRPC` | gRPC Server 地址 | `<test-host>:8085` |
| `IT_DRY_RUN` | 仅打印请求不真实下发 | `1` / `0` |

> 集成测试环境变量**必须从 vault / CI secret / 本地 `.env.local`（已 gitignore）注入**，禁止写入仓库。

---

## 二、用例设计与组织

### 1. 通用强制条款（红线）

| # | 红线 |
|---|------|
| 1 | **禁止**指向生产环境 / 共享线上库；只跑测试或 staging 环境 |
| 2 | **禁止**在 IT 中使用真实用户的真实凭证；用专用测试账号 |
| 3 | **禁止**测试遗留脏数据：每个用例自带 setup / teardown，或全局清理钩子保证幂等 |
| 4 | **禁止**用例间共享可变状态 / 依赖执行顺序（除非显式声明 `@order` 并文档化） |
| 5 | **禁止**硬编码环境信息（域名 / 路径 / 凭证 / 资源 ID） |
| 6 | **必须**记录 `req_id` / `trace_id` 到日志或报告，便于事后追溯 |
| 7 | **必须**为可能产生费用 / 不可逆操作的用例加保护开关（`IT_DRY_RUN` / 显式 confirm） |

### 2. 用例类型与适用场景

| 类型 | 适用 | 文件命名 / 位置 | 示例风格 |
|------|------|----------------|---------|
| 接口正向用例 | 验证主流程 happy path | `<package>/<module>_integration_test.go`（`//go:build integration`） | `TestXxx_Normal` |
| 接口逆向用例 | 参数错误 / 鉴权失败 / 资源不存在 | 同上，独立 `TestXxx_Negative` 函数 | `TestXxx_Negative` |
| 端到端链路用例 | 跨服务 / 跨模块流程 | `it/scenarios/<scenario>_integration_test.go` | `TestScenarioXxx` |
| 数据一致性用例 | DB / 缓存 / 副本数据校验 | `<package>/<module>_integration_test.go` 内 `TestXxx_DataConsistency` | `TestXxx_DataConsistency` |
| 异步 / 时序用例 | MQ 消费 / 回调 / 定时任务 | `<package>/<module>_integration_test.go` 内 `TestXxx_Async` | `TestXxx_Async` |

### 3. 文件命名硬约束

- 集成测试文件**用项目已有 build tag（如 `integration` 或 `it`）隔离**，普通 `go test ./...` 不编译、不运行，避免污染 UT 与拖慢 CI
- 集成测试文件与被测代码**同 package 同目录**（`<module>_integration_test.go`），或集中放 `it/` 包；不要与 UT 混在同一文件
- 每个被测接口 / 场景**独立 `TestXxx` 函数**，命名直观体现"测什么 + 正向/逆向"
- go-zero 项目：通过 `svc.ServiceContext` 注入测试用真实依赖（testcontainer 起的 mysql/redis），复用 `TestMain` 统一启停

### 4. 用例设计自检清单（四维度）

> 每个用例必须明确以下四个维度的设定，避免"测了个寂寞"。

| 维度 | 检查点 |
|------|-------|
| **输入** | 必填 / 选填 / 边界值 / 非法输入是否覆盖 |
| **状态** | 前置状态准备（DB 行 / 资源已创建 / 上游已 mock）是否清晰 |
| **依赖** | 上下游依赖（见 `relationship.md`）哪些需要真实、哪些可桩 |
| **断言** | 不仅断言响应码，还要断言核心字段、副作用（DB 写入 / 消息发送 / 日志输出） |

### 5. Mock / 桩策略

> 集成测试默认**真实调用**，但仍有需要桩的场景，避免连不上 / 不可控的依赖把 IT 变红。

| 类别 | 默认策略 |
|------|---------|
| 本项目核心接口 | **真实调用** |
| 同测试环境内的下游服务 | **真实调用** |
| 跨域 / 第三方 SDK（短信 / 支付 / 邮件） | 接入第三方提供的 sandbox / 测试模式；无 sandbox 时使用本地桩 |
| 不可控外部（如运营商 / 真实物理设备） | 必须桩；用例标注 `@requires_real` 显式排除 |
| 时间 / 随机 | 谨慎使用；如必要，用专用 fixture 隔离 |

### 6. 与项目 rules 承接

- 本仓库当前无 `.codebuddy/rules/integration_test_*.md` 等项目规则文件：本文规范即最低基线
- 后续如新增项目 rules，本文 §二 改为指向该 rules 锚点，不重复写
- 与 `.harness/plans/_template/04-it.md`（IT 阶段产物模板）配套使用：本文写"规则"，04-it.md 写"本次任务的执行结果"

### 7. ChainWeaver Test 组件用例（使用 CWT 的项目）

- `it/scenarios/e2e/` 编写本组件的完整业务链路；`it/scenarios/<模块>/` 编写接口正向、逆向、边界和状态变化场景，沿用项目已有组织方式。
- 新增或删除场景时，同步项目运行入口和全量场景清单；对照实际目录核对命令选中的包。
- 复用 CWT API 客户端、Service 业务能力和项目已有测试辅助函数；业务步骤、断言、资源清理由本项目 `it/` 维护，不搬入 CWT Service 或 `systest/`。
- 通过真实 HTTP / RPC 调用验证响应和写后读结果；两种入口均受影响时分别验证。明确当前身份、前置对象及清理责任，不依赖其它用例留下的数据。
- 测试连接使用 04 IT 的部署回执；采用 CWT 环境凭证的项目通过 `CHAINWEAVER_ENV` 注入，由项目的连接装配代码解析。测试用例关闭自身连接，环境申请和释放由外层按 `devops/test-env-deploy.md` 的 HTTP 调用负责。

### 8. CWT 异步适配

适用于本次需求影响已有 CWT API / Service 调用，或有明确下游场景需要新增调用能力时。先检查项目已有配置和调用入口，复用其 CWT 地址与调用方式。

1. 向 `POST /api/v1/api-sync-tasks` 提交 JSON `{"prompt":"..."}`。prompt 分别说明：
   - **接口契约**：新增或更新、准确路径与 HTTP 方法、请求 / 响应、业务码、鉴权及必要 diff；区分已发布与本地未发布契约，不发送真实凭证。HTTP DTO 以实际 JSON 编码为准，提供脱敏响应样本或服务端编码依据，特别核对时间戳、数字字符串、枚举和空值，不直接照搬 protobuf 字段类型。
   - **已有调用影响**：项目实际使用的 CWT API、Service 方法和受影响场景；适配已有 Service 时保留原业务职责，不扩大副作用。
   - **新增 Service 场景（需要时）**：当前身份、已有前置对象、业务动作、输入、结果及清理责任；调用示例供参考，不强制不合理签名。没有明确场景，不要求逐个 API 包装 Service。
2. 适配范围仅限 CWT API、Service 及关联单测，不修改 CWT `systest` 主流程，不要求隐式登录、申请环境、准备租户或自动删除业务资源。契约不足时补充资料，不让 Agent 猜测或保留静默兼容逻辑。
3. 收到 HTTP 202 和 `id` 后，将 id 记录到 `03-implement.md`，继续本地实现和 UT。409 表示已有任务执行中，本次未受理；POST 响应丢失时记录不确定性，通过服务端任务记录找回原任务，不盲目重发。当前接口没有任务列表或按 prompt 找回接口，无法找回时请服务维护者协助。
4. 本地实现完成后通过 `GET /api/v1/api-sync-tasks/{id}` 有间隔地轮询。`running` 继续等待；本地等待超时或中断时保留 id，后续继续查询。`failed` 展示 `error`、`questions`、`validation`，不使用该任务版本。
   - 查询遇到临时网络错误时，间隔重试同一 id；任务已明确失败或终止后，排除工具故障、补齐契约或按独立业务拆小批次，再提交新任务，记录前后 id 和失败原因。409、仍在运行或提交结果不明时不重复 POST；同一原因反复失败且无修正进展时暂停并报告，不无限重试。
5. 只有 `succeeded` 且返回 commit 才进入依赖交接：核对 `summary`、`files`、`validation` 和准确提交内容，按项目现有方式执行 `go get <CWT模块路径>@<返回commit>`，处理 `go.mod` / `go.sum` 及项目使用的 vendor。成功但缺少 commit 时停止依赖更新。
   - 更新后确认实际解析的依赖版本及 vendor 与该 commit 一致，审查受影响 API / DTO / Service 的 diff，并用服务端编码或脱敏真实响应独立核对契约，运行解码回归、受影响调用测试和编译；不能只依赖适配 Agent 同时生成的实现与单测。
   - 核验不通过时，将错误、契约依据及失败样本交回 API Sync 修正；取得新的成功 commit 后重新更新依赖并复核，核验通过才继续实现本项目 IT 场景。缺少契约证据或修正无进展时报告阻塞，不把依赖更新成功当作适配完成；核验结果记录到 `03-implement.md`。
6. 项目已有自动调用代码时，在其本地测试中覆盖 202 与 id 保存、查询重试与中断后续查、failed 后修正重提、409、POST 响应丢失及成功 commit 交接、依赖核验失败阻止后续步骤；更新现有调用说明。不为适配 SDK 删除原断言或恢复逻辑。CWT 返回的本地验证通过不代表本项目真实 IT 通过。

---

## 三、运行与调试

### 1. 标准执行命令

```bash
# 全量集成测试（CI 主入口；必须带 integration build tag）
go test -tags=integration -race ./...
# 本项目暂无 make it 封装 <!-- TODO(sop.init) 首批 IT 用例落地时补 make it target -->

# 指定 package
go test -tags=integration ./internal/logic/...

# 指定单个用例
go test -tags=integration -run 'TestXxx_Normal' ./internal/logic/

# Dry-run（不真实下发，仅校验拼接的请求）
IT_DRY_RUN=1 go test -tags=integration -run 'TestXxx' ./...

# 仅重跑失败用例
go test -tags=integration -run 'TestXxx' ./... -count=1

# 带 trace / 详细日志
go test -tags=integration -v -run 'TestXxx' ./... 2>&1 | grep -E "PASS|FAIL|req_id|trace_id"
```

> **必须**与 CI 中跑的命令一致；本地能过、CI 失败的多半是环境变量 / 网络访问 / 权限差异。

> 使用 CWT 的组件项目按本仓 Makefile 执行 `it/` 场景。例如已有入口为 `make it SCENARIOS="e2e user"`、`make it-all` 时，先执行受影响场景，再执行 Plan 约定的回归；命令、模块名和 build tag 以本项目实现为准。

### 2. 测试 runner 通常做了什么（如有 wrapper 脚本）

> 例：`run_it.sh` 通常负责：
> 1. 检查测试环境可达性（healthz / readiness）
> 2. 注入测试 token / endpoint 环境变量
> 3. 创建本次运行的隔离命名空间（`it-<run-id>`）
> 4. 调用 runner 执行用例
> 5. 失败时自动收集：失败用例的 `req_id` 列表、关键日志片段、环境快照
> 6. 结束时按隔离策略清理资源（即使失败也清理；保留产物到 artifacts/）

### 3. 调试套路

| 现象 | 优先排查 | 排查命令 |
|------|---------|---------|
| 全量失败 | 环境可达性 / 鉴权 / 配置 | 走 § 一的 5 项检查清单 |
| 全量超时 | 网络 / DNS / 测试环境过载 | `curl -v <endpoint>/healthz`；查测试环境监控 |
| 偶发失败（flaky） | 异步时序 / 资源竞争 / 依赖抖动 | 多次 `--lf` 重跑；检查是否依赖固定时序 |
| 单接口失败 | 抓取 req_id 查上下游链路日志 | 见 § 4 故障定位流程 |
| 用例间互相影响 | 数据隔离失效 / 全局状态 | 检查 setup/teardown / 命名空间隔离 |
| 鉴权失败 | token 过期 / 权限不足 | 重新签发 token；核对账号权限 |

### 4. 故障定位流程（接口级 IT 通用）

```
失败用例 → 取 req_id / trace_id
    → 查本服务日志（按 req_id 过滤）
    → 看上下游调用链（trace_id 串联）
    → 定位故障点（本服务 / 下游 / 数据问题）
    → 修复 / 跟进 owner
    → 重跑该用例确认
```

> 失败处理原则与 `_template/04-it.md` 一致：环境问题 → 修复后重跑；代码问题 → 回 03-implement；设计问题 → 回 02-plan。

### 5. 不要 / 慎用

| 项 | 原因 |
|----|------|
| 在 IT 中使用 `time.sleep` 等死等 | 改用 `wait_until(condition, timeout)` 显式轮询 |
| 共用同一份测试数据不清理 | 数据污染会让后续用例不可重复 |
| 用例依赖前一个用例创建的资源 | 用例必须独立；如需链路，用单一用例覆盖完整链路 |
| 关闭某些用例（`@skip` / `xfail`）以"让 CI 过" | 必须在 `04-it.md` 备注原因 + 跟踪修复 |
| 把 IT 和 UT 混到同一个 runner | 跑得慢、失败原因混杂、报告难读 |

### 6. 测试产物

| 产物 | 路径 | 用途 |
|------|------|------|
| JSON 测试报告 | `it_report.json`（`go test -json`） | CI 摄取（流水线展示） |
| 失败用例 trace 包 | `artifacts/failed/`（含 req_id、日志片段、租户 ID） | 事后追溯 |
| 终端摘要 | stdout | 通过 / 失败 / 跳过统计 |

### 7. CI 集成

- IT 在 CI 中**应与 UT 分离**为独立 stage / job：UT 快、阻塞 PR；IT 慢，可选阻塞或定时跑
- IT 失败必须**强制阻塞合入主干**（feature 分支可放宽，但需评审豁免）
- IT 失败后 CI 必须自动归档：`req_id` 列表、关键日志、环境快照（保留 ≥ 7 天）
- 测试环境部署 / 同步流程见 `.harness/docs/devops/test-env-deploy.md`

---

## 四、与 SOP 阶段的关系

| 阶段 | 引用本文的方式 |
|------|---------------|
| **02 Plan** | 设计 IT 用例清单时，按 § 二的"四维度自检"框架列用例；命名遵循 § 二规则 |
| **03 Implement** | 按 Plan 实现 `it/` 场景；涉及 CWT 适配时按 §二 §8 完成异步任务与依赖交接 |
| **04 IT** | 先按 `.harness/docs/devops/test-env-deploy.md` 完成部署并确认 § 一 §2「测试环境信息」可达；再按 § 三的标准命令执行，失败定位走 § 三 §4，产物按 § 三 §6 收集到 `04-it.md` |
| **03 Implement 的 Docs** | 若本次任务新增 / 修改了 IT 用例约定 / 环境配置，同步本文档 |
