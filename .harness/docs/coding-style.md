# 编码规范

> 状态：生效 | 维护者：jackz | 最后更新：2026-11-23（sop.init 裁剪为 Go + go-zero，阈值对齐 `.golangci.yml`）

## 范围

适用于本仓库所有 **Go** 代码（主服务）与 **TypeScript/React** 代码（`web/` 前端，走 ESLint/Prettier，见 `web/` 内配置）。本文主体针对 Go；shell 脚本遵循 `scripts/` 既有风格。

> 安全相关条款（输入验证、加密、并发安全等）见本文件 §10「安全编码」。接口层安全（鉴权、CORS、SSRF）见 [api-standards.md](apis/api-standards.md) §安全。

## 1. 强制规范（违反 = 阻断合入）

| # | 项 | 阈值 | 例外 |
|---|----|------|------|
| 1 | 格式化 | `gofmt` / `goimports` 通过 | 无 |
| 2 | 静态检查 | `golangci-lint`（`make lint`）通过；配置 `.golangci.yml`（staticcheck/revive/gocyclo/lll/goconst/misspell 等） | 必须用 `//nolint:<linter> // 中文理由` |
| 3 | 行长 | ≤120 列（lll 配置） | 生成代码、长字符串、struct tag、URL |
| 4 | 文件长度 | ≤800 行 | 测试文件 ≤1600 行 |
| 5 | 函数长度 | ≤80 行（revive function-length 配置） | 测试函数 ≤160 行 |
| 6 | 嵌套深度 | ≤4 层 | 无 |
| 7 | 圈复杂度 | ≤20（gocyclo min-complexity 配置） | 无 |
| 8 | 缩进 | gofmt（tab）；**禁止**空格与制表符混用 | 无 |
| 9 | 魔法数字 | 必须命名为常量 | 循环 0/1、退出码 1、判别式中的 4 |

## 2. 命名规范

### 通用原则
- 名称语义清晰、避免简写（除非是行业通用：URL、ID、API、UUID）
- **禁止**包名/类名：`common`、`util`、`misc`、`global`（多级目录如 `xx/util/encryption` 允许）
- **禁止**接收器/方法参数：`me`、`this`、`self`

### 各语言约定（本项目为 Go + 前端 TS）

**Go**
- 包名：全小写，单词，不带下划线，与目录同名
- 文件名：`snake_case`
- 类型名：`PascalCase`；接收器 1-2 字母缩写保持一致
- 函数/方法：`camelCase`（私）/ `PascalCase`（导出）
- 接口：单方法 `er` 后缀（Reader/Writer）；多方法用名词
- 缩写词：私有变量首词全小写（`apiClient`），其他保持原写法（`URLArray`、`UserID`）

**TypeScript（web/）**
- 组件 `PascalCase`；hooks `useXxx`；ESLint + Prettier（`web/` 内配置为准）

### go-zero 框架分层约定（强制）

- 目录分层 `handler → logic → svc → model` 单向依赖；handler 文件 `xxxHandler`、logic 文件 `xxxLogic` 命名
- 所有依赖经 `svc.ServiceContext` 注入，**禁止**在 logic 中直接实例化外部依赖
- 配置结构体在 `internal/config/` 定义，经 `etc/*.yaml` 加载（struct tag `json:",default=..."` 的 SA5008 告警可用 `//nolint:staticcheck` 定向豁免）
- 错误统一走 `internal/errcode`（HTTP）/ `internal/code`（gRPC），禁止裸 `error` 返回给前端
- 生成代码（goctl handler 框架 / `internal/types` / `pb/`）不手改不写测试

## 3. 注释规范

- **必须**：所有导出符号有文档注释，格式 `// 名称 描述`（Go）/ docstring（Python）/ Javadoc（Java）/ Doxygen（C++）
- **必须**：解释 **Why**，不是 **What**——What 应由代码本身表达
- **禁止**：保留被注释掉的代码（除非附 `TODO(name): 删除时机` 注释）
- 关键业务逻辑使用中文注释（团队默认语言）
- 异常/错误：使用 `@throws` / `@raises` 标注（Java/Python）

## 4. Import 规范

- 三组分隔（用空行）：标准库 → 第三方 → 本地包
- **禁止**相对路径 import（如 `from ..utils import x`）
- **禁止**点导入 `import .` 和裸空白导入 `import _`（main、test 例外，且需注释说明）
- 别名：包名冲突或与目录不一致时使用，命名遵循包命名规范
- 工具：`goimports` 自动整理（golangci-lint 已含）

## 5. 错误处理

### 通用
- 错误必须处理或显式忽略（`_ = ignored`，并附理由注释）
- 错误信息：小写开头，无标点结尾，格式 `module: 描述`
- **禁止**用 `panic` / `throw` / `os.exit()` 控制业务流程

### Go
- 使用 `errors.New("xxx")` 或 `fmt.Errorf("module: %w", err)` 包装错误
- error 必须是函数最后一个返回值
- `panic` 仅用于不变量断言、`init` 失败、`MustXXX` 工厂；**禁止**用于业务错误
- HTTP 错误统一 `BizError`（`internal/errcode`），gRPC 错误统一 `RespCode`（`internal/code`），禁止字符串比对判错

## 6. 资源管理

- 申请资源后**立即**写释放代码（Go: `defer` / Python: `with` / Java: try-with-resources / C++: RAII）
- **禁止**在循环中使用 `defer`（Go）
- 文件、连接、锁、信号量必须配对释放
- C++ 必须使用智能指针（`unique_ptr` / `shared_ptr`），**禁止**裸 `new` / `delete`

## 7. 并发与并行

- 共享状态必须有保护：`sync.Mutex` / `sync.RWMutex` / `sync.Map` / `Atomic`
- **禁止**并发写 map（Go）；**禁止**重复关闭 channel
- 闭包捕获循环变量必须通过参数或局部变量传递
- 64 位数值（long/double）读写必须保证原子性（Java 用 volatile/Atomic）
- 必须确保协程/线程能退出（避免泄漏）：超时、退出信号、context 取消
- 死锁防护：固定加锁顺序

## 8. 类型与零值

- 空切片/列表用 `nil` / `None`，不用 `[]` 初始化（Go）
- 预分配 map/slice/list 容量以减少扩容
- 字符串拼接：循环内避免 `+`，使用 `StringBuilder`（Java）/ `strings.Builder`（Go）/ `''.join()`（Python）
- 浮点数比较：使用 epsilon，**禁止** `==`

## 9. 代码风格

### 函数与方法
- 参数数量 ≤5 个；超出考虑结构体/对象封装
- 命名返回值：仅在返回 2-3 个相同类型或上下文不清晰时使用（Go）
- 方法名不重复接收器/参数/返回值的语义信息
- 倾向值传递；map/slice/chan/interface 不要传指针（Go）

### 控制流
- **优先早返回**（early return），减少嵌套
- if-else 链超过 3 段考虑改 switch / 策略模式
- switch 必须有 `default`（Go）

### 变量声明
- 就近原则：声明在第一次使用前
- 局部变量优先短命名；作用域大则名称应有描述性

## 10. 安全编码（语言层）

> 接口层安全（鉴权、CORS、SSRF、CSRF）见 [api-standards.md](api-standards.md)。

### 输入与拼接
- **【必须】** 数据库查询使用预编译/参数化语句；**禁止**字符串拼接 SQL
- **【必须】** ORDER BY、表名等无法参数化的字段使用白名单校验
- **【必须】** 命令执行使用结构化 API（`exec.Command` 数组形式 / `subprocess.run([...], shell=False)`），**禁止**拼接用户输入到 shell
- **【必须】** 路径操作使用 `filepath.Clean()` + `startsWith()` 校验，**禁止**容忍 `..`
- **【必须】** 外部输入白名单校验：类型、长度、范围、格式
- **【必须】** HTML 输出对第三方数据编码（`html.EscapeString` / `bleach.clean`）

### 密钥与敏感信息
- **【禁止】** 硬编码 AK/SK、密码、密钥、内部 IP、员工信息
- **【必须】** 通过配置中心、环境变量、KMS 注入敏感配置
- **【必须】** 日志脱敏：身份证显示首尾、手机号隐藏中 6 位、银行卡仅显示后 4 位
- **【禁止】** 日志输出：密码、密钥、Token、指针值
- **【必须】** 加密：使用 SHA-2 / RSA / AES-GCM；**禁用** DES、3DES、MD5、SHA1、RC4
- **【必须】** 密码哈希：随机盐 + 多轮（bcrypt / scrypt / argon2id）
- C++：敏感数据使用后立即清零（`OPENSSL_cleanse()` / `SecureZeroMemory()`）

### 随机数
- **【禁止】** `rand()` / `math/random` 用于安全场景
- **【必须】** 使用 CSPRNG：`crypto/rand`（Go）、`secrets`（Python）、`SecureRandom`（Java）、`RAND_bytes`（OpenSSL/C++）
- **【禁止】** `rand.Int() % n` 形式，使用 `rand.Intn(n)`

### TLS 与传输
- **【必须】** TLS ≥1.2，推荐 TLS 1.3；**禁止**明文传输敏感数据
- **【必须】** 启用证书校验；**禁止** `InsecureSkipVerify: true`（除非测试环境且代码标注）

## 11. 工具链

| 工具 | 用途 | 触发时机 |
|------|------|----------|
| `gofmt` / `goimports` | 自动格式化 | 保存 / pre-commit / golangci-lint |
| `golangci-lint`（`make lint`） | 静态检查（staticcheck / revive / gocyclo / lll 等 15 个 linter） | pre-push / CI |
| `go vet` | 基础静态检查 | 随 golangci-lint |
| `go test -cover`（`make ut`） | 覆盖率报告 | CI |
| `gocloc`（`make comment`） | 注释率统计 | 手动 |

## 12. 例外与豁免流程

如有充分理由违反上述规范：
1. 在代码处加 `//nolint:<linter> // 中文理由`（Go 惯例，参考 `internal/sdk/types.go:20`、`internal/config/config.go` 既有用法）并写明理由
2. 在 PR 描述中说明影响面与缓解措施
3. Reviewer 二次确认

## 参考

- 项目级语言规范：`.golangci.yml`（本仓库根，linter 配置即机器可执行规范）
- 测试断言规范：[unittest/unittest.md](unittest/unittest.md)
- API 与接口安全：[api-standards.md](apis/api-standards.md)
- Code Review 检查清单：[code-review.md](code-review.md)
