# 单元测试规范（Go 环境 / 生成 / 运行调试）

> 让任意成员（或 AI Agent）能在本地把 Go 单元测试**装得起、写得对、跑得通、出错查得到**。
> 三段式结构：① 环境搭建 / ② 生成规范 / ③ 运行调试。
>
> **若仓库存在项目 unittest 规则文件**（`.codebuddy/rules/unittest_*.md`、`.cursor/rules/unittest_*.md`、`.windsurf/rules/` 等，见 `AGENTS.md` 文末「项目既有规则」），那是权威来源：本文以摘录 + 链接形式承接，不另起冲突；调整本文必须同步 rules。

> Source: go.mod（Go 1.22.7 / testify v1.9.0）+ Makefile（ut / gen-mock）+ 现有 12 个 `*_test.go`
> Last-verified: 2026-11-23

---

## 〇、本项目测试现状（sop.init 探测）

- 现有测试文件 12 个：`internal/billing`(1)、`internal/errcode`(1)、`internal/middleware`(3: anomaly/audit/ratelimit)、`internal/sdk`(6: chainmaker/ethereum/solana_codec/solana_process/solanaclient)、`internal/util`(1)、`internal/validator`(1)
- `testify` 已在 go.mod 但现有测试未使用；`make gen-mock`（mockgen 生成 `chaininteractive` mock）存在但 `mock/` 目录未落地
- `scripts/ut_cover.sh` 的覆盖率门禁（40%）与目录检查已被注释（历史脚本指向已不存在的 `internal/dao`、`internal/logic` 目录）——CI 覆盖率门禁以本文 §三.6 为准
- 运行命令：`make ut`（= `go test -coverprofile cover.out ./...`）

### go-zero 框架特化（必须遵守）

- **logic 层是核心测试目标**；handler 层（goctl 生成）不单独测
- 生成代码（`internal/handler` 框架、`internal/types`、`pb/`）**不写测试**，只测 logic 业务逻辑
- 对 `svc.ServiceContext` 中的依赖做 interface 抽象后 mock（本项目首选抽象 `Repository`、`ChainPlugin` 两大接口）
- 完整 handler 链路测试用 `httptest`

---

## 一、环境搭建与依赖安装

### 0. 推荐技术选型（Go）

| 类别 | 推荐 | 备选 |
|------|------|------|
| 构建 / 测试 runner | **`go test`**（标准库自带，零依赖）| — |
| 断言库 | **`github.com/stretchr/testify`**（assert + require）| 标准库 `testing` |
| Mock 框架 | **`go.uber.org/mock`（gomock）** 或 **`github.com/stretchr/testify/mock`** | 手写接口实现 |
| HTTP 测试 | **`net/http/httptest`**（标准库自带）| — |
| SQL/DB 测试 | **`github.com/DATA-DOG/go-sqlmock`** | `pgx` / `database/sql` 真实库（集成测试）|
| 覆盖率 | **`go test -cover` / `-coverprofile`**（标准库自带）| `gocov` |
| 模糊测试 | **`go test -fuzz`**（标准库 1.18+）| — |
| 基准测试 | **`go test -bench`**（标准库自带）| `github.com/onsi/ginkgo/v2` |

> **为什么默认只信赖标准库 + testify**：
> - `go test` 是 Go 一等公民，无需额外 runner；表驱动测试（table-driven）是其官方推荐范式
> - `testify/assert` 失败时报告差异点，`require` 在失败后 `t.FailNow()` 避免后续断言雪崩
> - `go.uber.org/mock` 由 Google 维护、与 `go generate` 集成自动生成 Mock，契合 Go 的接口即契约哲学

### 1. 前置依赖检查清单（先查再装）

> 任何一项已满足就跳过对应安装步骤，不要无脑 `go clean` 重建。

```bash
# (1) Go 工具链
command -v go >/dev/null && go version || echo "需要安装 Go"

# (2) 项目所需 Go 版本（与 go.mod go 指令一致）
grep -E "^go +[0-9]" go.mod | awk '{print $2}' | xargs -I{} go version | grep -F "{}" \
  >/dev/null && echo "Go version OK" || echo "需要切换 Go 版本"

# (3) 关键测试依赖
go list -m github.com/stretchr/testify >/dev/null 2>&1 && echo "testify OK" || echo "缺少 testify"
go list -m go.uber.org/mock >/dev/null 2>&1 && echo "mock OK" || echo "缺少 mock"

# (4) 编译产物 / 生成代码（mock_gen、protobuf 等，按需）
test -f <生成的 mock 文件> && echo "mocks OK" || echo "需要生成 mock"
```

判断逻辑：4 项全过 → 直接执行测试；某项失败 → **只修复失败项**，不要全量重建（除非 Go 版本不对）。

### 2. 分步安装（仅在上面检查失败时执行）

#### Step 1: 安装 Go 工具链

```bash
# macOS
brew install go

# Linux / 通用：从 https://go.dev/dl/ 下载对应版本，或使用 gvm / asdf 管理多版本
# 写入 shell 配置（仅首次）
export GOROOT="$(go env GOROOT)"
export GOPATH="$HOME/go"
export PATH="$GOPATH/bin:$PATH"
```

#### Step 2: 锁定项目 Go 版本

```bash
# 以 go.mod 中的 go 指令为准（例：go 1.22）
go mod edit -go=1.22
go mod tidy
```

#### Step 3: 安装测试依赖

```bash
go get github.com/stretchr/testify@latest
go get go.uber.org/mock/gomock@latest
go get github.com/DATA-DOG/go-sqlmock@latest
go install go.uber.org/mock/mockgen@latest   # mock 代码生成器
```

#### Step 4: 生成 Mock 代码（如依赖接口）

```bash
# 本项目接口 mock（Makefile gen-mock 现有命令）
mockgen -source=chaininteractive/chaininteractive.go -destination=mock/mock_chain_interactive.go -package=mock
# 建议按需扩展：
mockgen -source=internal/store/repository.go -destination=internal/store/mock/mock_repository.go -package=mock
mockgen -source=internal/sdk/interface.go -destination=internal/sdk/mock/mock_chain_plugin.go -package=mock
# 或在接口上加 //go:generate mockgen ... 后用 go generate ./...
```

#### Step 5: 一键脚本（如有）

> 项目通常会提供 `make test-setup` / `bash scripts/setup_test.sh` 等。**优先用一键脚本**，本节是其语义说明。

### 3. 环境变量与配置

| 变量 | 用途 | 示例 |
|------|------|------|
| `GODEBUG` | 调试调度 / GC 行为（按需）| `GODEBUG=gctrace=1` |
| `TEST_LOG_LEVEL` | 测试日志级别，内部 `log.SetLevel` 读取 | `TEST_LOG_LEVEL=debug` |
| `MOCK_FLAG` | 强制启用本地 mock，禁止真实外部调用 | `MOCK_FLAG=1` |
| `GOFLAGS` | 注入 `-mod=mod` 等全局 flag | `GOFLAGS=-mod=mod` |

> 测试相关环境变量必须**在测试代码内或 Makefile target 中设置**（如 `TestMain` 里 `os.Setenv`），避免成员手动 export。

---

## 二、单元测试生成规范

### 1. 通用强制条款（红线）

| # | 红线 |
|---|------|
| 1 | 不修改被测业务代码以让测试通过 |
| 2 | 不调用真实外部服务 / 真实写库；数据库用 `go-sqlmock`、HTTP 用 `httptest`、消息用内存实现 |
| 3 | 不在测试间共享可变状态 / 不依赖执行顺序；每个用例 `t.Run` 子测试独立 |
| 4 | 不删除已有测试代码，只追加 |
| 5 | 新 Mock / helper / fixture 只能追加到对应文件**尾部** |
| 6 | 不硬编码环境信息（域名 / 路径 / 凭证）；用 `t.TempDir()` 代替固定临时路径 |
| 7 | 禁止 `gomonkey` / `monkey` 等运行时 patch 生产函数（破坏可移植性，CI 易崩）；依赖应抽象为接口后 mock |
| 8 | 不在测试里 `t.Parallel()` 与共享可变全局状态混用，避免数据竞争 |

### 2. 测试类型与适用场景

| 测试类型 | 适用 | 文件命名 / 位置 | 风格 |
|---------|------|----------------|------|
| 函数级单元测试 | 单个 / 少量函数小改 | `xxx_test.go`（与被测 `xxx.go` 同包同目录）| 表驱动 |
| 接口 / 入口级测试（handler/service）| 端到端入口流程 | `xxx_test.go` + `httptest` | 表驱动 + Mock 依赖 |
| 模糊测试 | 解析 / 反序列化等输入不可信路径 | `FuzzXxx` 函数 | `go test -fuzz` |
| 基准测试 | 性能敏感路径 | `BenchmarkXxx` 函数 | `b.ResetTimer()` |

### 3. 文件命名与组织硬约束

- 测试文件**与被测代码文件同包同目录**，`_test.go` 后缀；内部测试用白盒（`package xxx`），仅测导出 API 且需黑盒时用外部测试（`package xxx_test`）
- 同一被测文件的多个函数 / 多个场景测试**统一放进同一个 `xxx_test.go`**；新增 → 追加，不新建并列文件
- 表驱动：一个 `TestXxx` 内用 `tests := []struct{...}{...}` + `for _, tt := range tests { t.Run(tt.name, ...) }` 覆盖多用例
- Mock 代码放 `mocks` 子包或 `mock_<pkg>` 包，由 `mockgen` 生成，**不手写维护**

### 4. Mock 策略（Go 接口即契约）

| 类别 | 默认策略 |
|------|---------|
| 项目内函数（utils / 校验器 / 纯函数）| 默认允许真实调用（不 mock）|
| 外部依赖（DB / HTTP client / MQ / 文件系统 / 第三方 SDK）| 必须抽象为**接口**后用 gomock / testify-mock |
| 可观测性（日志 / metrics）| 不 mock；用 `t.Log` / 内存 sink 验证 |
| 时间 / 随机 | 注入 `Clock` 接口或 `func() time.Time` 字段，测试里固定 |
| 全局配置 / 常量 | 通过依赖注入（构造函数参数）传入，不依赖包级变量 |

> **依赖注入范式**：被测结构体把依赖作为接口字段，测试注入 mock 实现。避免包级 `var db *sql.DB` 这类硬编码依赖——它无法 mock。

### 5. 表驱动测试用例设计自检清单

> 详见 `plans/_template/03-implement.md` § 用例设计清单。每个用例至少覆盖：
> - **正常路径**：典型输入 → 期望输出
> - **边界值**：空切片 / nil / 零值 / 最大值
> - **错误路径**：依赖返回 error 时上层处理是否正确
> - **幂等性**：同一输入重复调用结果一致

### 6. 与项目 rules 承接

- 本仓库当前无 `.codebuddy/rules/unittest_*.md` 等项目规则文件：本文规范即最低基线
- 后续如新增项目 rules，本文 §二 改为指向该 rules 锚点，不重复写

---

## 三、运行与调试规范

### 1. 标准执行命令

```bash
# 全量（当前模块及子模块）
go test ./...

# 指定包 / 文件
go test ./internal/service/...
go test ./internal/repo/user_test.go ./internal/repo/user.go

# 指定用例（正则匹配 Test 名 / 子测试名）
go test ./internal/service/ -run 'TestCreateUser/empty_name' -v

# 仅重跑上次失败用例
go test ./... -run '^(TestCreateUser|TestUpdateUser)$'

# 带覆盖率
go test ./... -cover
go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out

# 交互式查看未覆盖行
go tool cover -html=coverage.out

# 基准 / 模糊
go test ./... -bench=. -benchmem
go test ./... -fuzz=FuzzParse -fuzztime=30s
```

> **必须**与 CI 中跑的命令一致；本地能过、CI 失败的多半是环境变量 / 路径差异（如 `t.TempDir()` 在 CI 路径权限不同）。

### 2. 测试 runner 通常做了什么（如有 Makefile target）

> 例：`make test` 通常负责：
> 1. 设置必要环境变量（`TEST_LOG_LEVEL` 等）
> 2. 清理上次运行残留（`coverage.out` 等）
> 3. 调用 `go test ./... "$@"`，把命令行参数透传
> 4. 可选：`go vet ./...` + `golangci-lint run` 前置检查

### 3. 调试套路

| 现象 | 优先排查 | 排查命令 |
|------|---------|---------|
| 全量失败 | 环境 / 依赖 / Go 版本 | 走 § 一的 4 项检查清单 |
| 单文件失败 | 隔离复现 | `go test ./pkg -run TestXxx -v` |
| 偶发失败 | 并发竞争 / 共享状态 | `go test -race ./...` |
| 覆盖率不达标 | 看未覆盖行 | `go tool cover -func=coverage.out` |
| import 报错 | module 损坏 / 路径错 | `go list -m all | grep <pkg>` |
| Mock 没生效 | `EXPECT()` 未调用 / 顺序错 | `gomock` 报 `missing call(s)` → 检查 `ctrl.Finish()` / `defer ctrl.Finish()` |

### 4. 不要 / 慎用

| 项 | 原因 |
|----|------|
| 修改 `Makefile` / `go.mod` / CI 测试配置等共享配置 | 影响全员，需走 PR 评审 |
| 用 `gomonkey` 等运行时函数 patch | 破坏跨平台 / CI（macOS ARM 易崩），应用接口 mock 替代 |
| 在测试里 `fmt.Println` 大量调试信息 | 改用 `t.Log` / `-v` |
| 跳过用例（`t.Skip`）以"让 CI 过"| 必须在 `03-implement.md` 备注原因 + 跟踪修复 |
| 测试依赖真实网络 / 数据库 | 慢、不稳定、CI 无法复现；一律 mock |

### 5. 测试产物

| 产物 | 路径 | 用途 |
|------|------|------|
| 覆盖率 profile | `coverage.out` | `go tool cover` 分析 |
| 覆盖率 HTML | `coverage.html` | 本地交互式查看未覆盖行 |
| 基准结果 | stdout / `bench.out` | 性能回归对比 |
| 终端摘要 | stdout | 通过 / 失败 / 耗时 |

### 6. CI 集成

- CI 中跑测试必须**与本地命令一致**（仅可加 `-coverprofile` / `-race` 等输出参数）
- **强制 `-race` 检测数据竞争**（Go 并发项目必开）
- 失败必须阻塞合入
- **覆盖率门槛：单元测试覆盖率最低 60%**，低于 60% 必须阻塞合入；回退（低于 baseline 或较上次下降）触发警告
- 推荐 CI 命令：`go vet ./... && go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1`
