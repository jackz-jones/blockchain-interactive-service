# chain-interactive-service — AI 协作入口

> 本文件是 AI 协作的**唯一真源入口**（AGENTS.md）。CODEBUDDY.md / CLAUDE.md 均为指向本文件的软链接，所有 AI 工具通过软链读取同一份内容。
> 具体技术文档维护到 `.harness/docs/`，任务产物维护到 `.harness/plans/<任务目录>/`，本文件只做索引 + 恢复协议 + 项目特定红线。

**项目一句话**：多链（Ethereum / ChainMaker / Solana）区块链交互服务 BaaS 平台——统一 gRPC + RESTful API 屏蔽链差异，含多租户、计费配额、插件化链适配、审计与事件订阅。Go 1.22.7 + go-zero v1.6.2。

---

## 一、快速索引

| 想了解什么 | 文档位置 |
|-----------|---------|
| 项目整体架构、模块划分、关键流程 | `.harness/docs/architecture.md` |
| 上下游 / 集成方关系 | `.harness/docs/relationship.md` |
| 接口契约 / 文档 | `.harness/docs/apis/index.md`（索引 `proto/chaininteractive.proto` 与 `api/chaininteractive.api`） |
| 数据库 / 持久化 schema | `.harness/docs/sqls/` |
| **单元测试规范（环境 / 生成 / 运行调试，三段式）** | **`.harness/docs/unittest/unittest.md`** |
| **集成测试规范（环境 / 用例 / 运行调试，三段式）** | **`.harness/docs/integration_test/integration_test.md`** |
| 本地开发环境搭建与启动 | `.harness/docs/devops/development.md` |
| 测试环境部署 / 代码同步 | `.harness/docs/devops/test-env-deploy.md` |
| 部署与运行规范 | `.harness/docs/devops/deployment.md` |
| 编码规范（行长 / 复杂度 / 命名 / 安全编码） | `.harness/docs/coding-style.md` |
| 团队人工 Code Review 流程与合入门槛 | `.harness/docs/code-review.md` |
| 日志规范（级别 / 必打场景 / 脱敏 / 链路追踪） | `.harness/docs/logging.md` |
| 业务术语表 | `.harness/docs/glossary.md` |
| 踩坑记录 | `.harness/docs/failures.md` |
| AI Code Review 指导规范 | `.harness/review.md` |
| **Commit 规范（格式 / 执行顺序 / 一个 MR 一个 commit）** | **`.harness/commit.md`** |
| **小需求模式触发规则（排除项 / 行数参考线 / 漏估回退）** | **`.harness/small-requirement.md`** |
| 所有任务清单 | `ls .harness/plans/`（每个任务一个目录，互不干扰） |
| 单个任务的总览 & 进度（**单一真相源**） | `.harness/plans/YYYY-MM-DD_<title>/00-overview.md` |
| 单个任务的分阶段产物 | `.harness/plans/YYYY-MM-DD_<title>/0N-<step>.md` |

### Resource Map（按任务类型选读）

> 只读本任务需要的那几份领域文档，不整包读 `.harness/docs/`。

| 任务类型 | 必读 | 按需 |
|---------|------|------|
| 普通实现 / 重构 | `.harness/docs/coding-style.md` | `.harness/docs/architecture.md`、`.harness/docs/glossary.md` |
| 接口 / 契约变更 | `.harness/docs/apis/api-standards.md`、`.harness/docs/apis/index.md` | `.harness/docs/sqls/database.md`、`.harness/docs/relationship.md` |
| 数据 / 持久化变更 | `.harness/docs/sqls/database.md` | `.harness/docs/architecture.md` |
| UT / IT | `.harness/docs/unittest/unittest.md`、`.harness/docs/integration_test/integration_test.md` | `.harness/docs/devops/test-env-deploy.md` |
| 部署 / 环境 | `.harness/docs/devops/development.md`、`.harness/docs/devops/test-env-deploy.md` | `.harness/docs/devops/deployment.md` |
| 修复 / 评审 | `.harness/docs/failures.md`、`.harness/docs/code-review.md` | 受影响领域文档 |

### 常用命令

```bash
make gen-code        # 由 proto/.api 重新生成代码（scripts/generate_code.sh）
make build           # 编译二进制（版本 LDFLAGS 注入 internal/version.go）
make start-service   # 本地构建并运行服务
make lint            # golangci-lint（配置见根目录 .golangci.yml）
make ut              # go test -coverprofile cover.out ./...
make gen-mock        # mockgen 生成 chaininteractive mock
make gen-cert        # 生成 TLS 证书（scripts/generate_cert.sh）
make build-docker    # vendor 化 chainmaker SDK 后构建并推送镜像
cd web && npm run dev   # 前端本地开发（React 19 + Vite + antd）
```

### 开发环境要点与陷阱

- **双重 gRPC 拦截器**：Auth / RBAC / Quota 拦截器 Unary + Stream 两套都要挂（`chaininteractive.go`）。
- **配置含密钥**：`etc/chaininteractive.yaml` 内含 Redis 密码与 DB DSN，文档/日志中引用必须脱敏（见 `internal/validator/validator.go` 的 Mask 函数）。
- **`pb/` 包名为 `__`**（proto `go_package="./"` 所致），导入需别名；生成代码禁止手改。
- **路由真源是 `internal/handler/routes.go`**：它已手工改造（新增公开路由 / 事件订阅路由），与 `api/chaininteractive.api` 声明存在差异，改路由时两处都要核对。
- **Redis 部分依赖**：事件推送用 Redis Stream；限流是**进程内**滑动窗口（多副本不共享）；Leader 选举代码存在但未接线。

---

## 二、SOP（标准开发流程）

> 【本地化】本项目为 GitHub 开源项目：issue = GitHub Issue（`Closes #N` 合并自动关闭）、MR = Pull Request；commit 脚本校验 GitHub issue 引用（`scripts/commit_msg_check.sh`）。sop.init 升级会把本章刷回 TAPD 版，升级后需重做本地化。

> **触发（强制）**：以下任一情形进入 SOP，从「上下文恢复」开始：
> 1. 用户显式说「开始 SOP」「新建任务：<描述>」；
> 2. 用户提出任何需求 / 改动请求（功能 / 修复 / 配置 / 文档 / 接口调整……），**即使只是随口一句「把 X 改一下」**——先按 SOP 处理，按用户已明确的流程和授权执行；用户明确跳过时直接处理；
> 3. 版本迭代续改：历史任务已 commit 完成 / 已发布，又回到同一分支继续改动。
> **判定顺序**：上下文恢复 → 匹配 `.harness/plans/` 已有任务目录：
>    - 命中且未完成 → 在该任务继续（`01-clarify.md` 已有有效内容 → 跳过 Clarify 直接进 Plan，需用户确认 + 在 `00-overview.md` 标记跳过）；
>    - 命中但已完成 / 未命中 → 按「SOP 启动前置」新建任务目录（新需求开新分支，一个分支只对应一个任务目录）。
> **不触发**：纯答疑 / 代码阅读 / 文档查询等**不改动**场景，直接回答即可。

### 上下文恢复（每次会话/clear/compact 必做）

> 1. 读 `AGENTS.md` → 2. `git branch --show-current` → 3. 遍历 `.harness/plans/*/00-overview.md` 匹配分支 → 4. 读当前任务 `00-overview.md` → 5. Lazy-load 当前阶段 md → 6. 向用户汇报进展。
> 匹配不到 → 按「SOP 启动前置」处理。

### 任务隔离（强制）

> **严禁** AI 主动读取/参考**其他**任务目录 `.harness/plans/<其他任务>/` 下的任何 md。任务之间物理隔离、互为独立真相源，跨任务参考会污染设计判断。
> **唯一例外**：用户**显式**说「参考任务 X」→ 仅读指定目录，内容只留对话上下文，**禁止**自动写回当前任务产物。

### SOP 启动前置：分支与任务判断

> 创建任务时读取 `git config user.name` 填写 `00-overview.md` 的负责人，读取不到时请用户填写；模板中的负责人留空。

> 1. `git branch --show-current` 获取分支名
> 2. 在 `.harness/plans/` 匹配分支：
>    - **有对应任务** → 按 `00-overview.md` 继续
>    - **无 + 在项目主线** → 全新需求：`git checkout -b feature/<name> ${SOP_TARGET_BRANCH:-origin/master}` → `cp -r .harness/plans/_template .harness/plans/YYYY-MM-DD_<title>` → **精简注释**（见下方 ③）→ 填 Meta → 进入入口判定
>    - **无 + 在 feature/ 分支** → 「协同开发检测」
> 3. **复制 _template/ 后必须精简注释**（全新需求 / 协同开发通用，**AI 必执行**）：打开新建的 `plans/<task>/00-overview.md`，把所有 `<!-- TEMPLATE-ONLY-DO-NOT-COPY: -->` 标记的 HTML 注释块**整段删除**（含标记行），替换为单行指针指向 `_template/00-overview.md`；**头部 "⚠️ TEMPLATE ONLY" 段也整段删除**。SOP 规则只在 `_template/` 维护，**禁止**在每个任务文件里重复 ~30 行规则（噪音 + 版本漂移）。详见 `_template/00-overview.md` 顶部说明。
>
> **禁止**在项目主线上直接做 SOP；目标分支按 `SOP_TARGET_BRANCH` 或项目仓库约定确定；一个分支只允许对应一个任务目录。

### 协同开发检测（design.md 驱动）

> 触发：当前分支不是项目主线且无对应任务。
> - `.harness/design.md` 存在且用户确认使用 → 创建任务目录（同样按上方 ③ 精简 _template/ 注释）→ 从 design.md 完整派生 `01-clarify.md` + `02-plan.md`（禁止只写「详见 design.md」）→ 填 Meta（`开发模式`=协同、`测试环境`）→ 自检后删除 design.md → 从 Step 3 开始
> - design.md 不存在 → 标准流程

### 5 步骤定义

| # | 步骤 | 产物 | 说明 |
|---|------|------|------|
| 1 | Clarify | `01-clarify.md` | 澄清目标和边界 |
| 2 | Plan | `02-plan.md` | 方案、UT 和 IT 用例 |
| 3 | Implement | `03-implement.md` | 实现、UT 和文档更新 |
| 4 | IT | `04-it.md` | 通过项目已有入口部署、测试和环境收尾；不适用时记录原因 |
| 5 | Review & Commit | `05-review-commit.md` | 审查、GitHub Issue/PR、提交与 amend |

### 执行规则

> 已明确授权的普通步骤连续执行并汇报状态，不按步骤反复确认。范围、副作用或交付终点变化时再确认；commit、push、MR、合入和部署分别按授权执行。
> 各阶段只保留一个最终完成条件，记录真实开始、结束时间和进度；不适用、明确跳过、真实阻塞分别说明原因和剩余验证范围。04 验证本组件业务及原租约收尾，05 自审后提交，再核验远端与 GitHub PR。
> 已存在的旧任务按其原任务记录继续。恢复“交付待核验”任务先查询 Git 与 GitHub（PR、issue）的真实状态，再继续剩余工作，不重复创建提交或 PR。

---

## 三、开发准则

### 代码检测（CI 强制）

```bash
make lint          # golangci-lint（启用 staticcheck / revive / gocyclo / lll 等，见 .golangci.yml）
go vet ./...       # govet 内含于 golangci-lint
```

| 规则 | 要求 |
|------|------|
| 行长度 | ≤ 120（lll） |
| 复杂度 | gocyclo ≤ 20；revive 函数体 ≤ 80 行 |
| 未使用 import | goimports + unused 检查 |
| nolint 惯例 | 仅定向豁免并附中文理由（如 `//nolint:staticcheck` 针对 struct tag SA5008） |

### 语言 / 框架版本约束

| 组件 | 版本 |
|------|------|
| Go | 1.22.7（与 `go.mod` 一致；golangci target 1.21） |
| go-zero | v1.6.2 |
| GORM | v1.25.10（驱动 mysql/postgres/kingbase） |
| gRPC / Protobuf | grpc v1.65.0 / protobuf v1.34.2（生成器 protoc-gen-go v1.32.0） |
| 链 SDK | go-ethereum v1.14.11 / chainmaker-sdk-go v2.3.8 / solana-go v1.8.3 |

### go-zero 开发约定

- 分层 `handler → logic → svc → model` 单向依赖；依赖一律经 `svc.ServiceContext` 注入，禁止 logic 内直接实例化外部依赖。
- HTTP 请求/响应类型在 `.api` 文件定义（goctl 生成 `internal/types`），禁止手写 handler 层结构体；API 版本经 `prefix: /api/v1` 控制。
- 错误统一走 `internal/errcode`（`BizError` + 分段码），禁止裸 `error` 返回给前端。
- gRPC 拦截器（Auth/RBAC/Quota）Unary 与 Stream 两套同步维护。
- 生成代码（`internal/handler`、`internal/types`、`pb/`）不手改不写测试，只测 logic 业务逻辑。

### 安全基线

1. SQL 必须参数化（GORM where 占位），禁止字符串拼接。
2. 禁止硬编码密钥、Token、密码；`etc/*.yaml` 中已有凭据仅限本地/内网，严禁复制到文档或日志。
3. 日志输出敏感字段必须脱敏：私钥前 4 后 4（`MaskSensitiveString`）、API Key 前 8 后 4（`KeyMasked`）。
4. 外部输入必须校验（`internal/validator`），输出按场景转义。

---

## 四、禁止红线

| # | 红线 | 后果 |
|---|------|------|
| 1 | **严禁 AI 主动读取/参考其它 `.harness/plans/<其他任务目录>/` 下的 md 产物**（含 `00-overview.md`、`01-clarify.md` … `05-review-commit.md`、`.harness/design.md` 等）；仅当用户**显式**指定「参考任务 X」时才可读指定的那一个任务目录，且参考内容禁止自动写回当前任务。详见上文「任务隔离原则（强制）」章节。 | 任务单一真相源被污染；跨任务上下文干扰当前任务设计；与并发开发冲突 |
| 2 | **禁止手改生成代码**：`pb/`、`internal/handler/`（goctl 生成框架）、`internal/types/`；契约变更必须改 `proto/` 或 `.api` 后 `make gen-code` 重新生成。 | 生成物漂移，下次再生成覆盖手工改动 |
| 3 | **禁止把 `etc/` 下的密钥/DSN/私钥文件提交到新的提交或复制进文档/日志**（历史已提交的凭据见踩坑记录 `98bffb6c`）。 | 凭证泄露；hex 解析失败类故障 |
| 4 | **禁止在 gRPC 拦截器 / HTTP 中间件链上"只加 Unary 不加 Stream"或漏挂审计**：链上资源消耗操作必须挂 `QuotaMiddleware`（参考 routes.go L40-74 注释分区）。 | 鉴权/计费旁路，安全与营收漏洞 |
| 5 | **SDK 客户端内部 context 必须用 `context.Background()` 派生**，禁止透传 HTTP 请求 ctx 导致写链被取消（踩坑 `deeb1e03`）。 | 链上交易随请求取消而中断失败 |
