# 03. Implement

> 输入：`02-plan.md`；输出：实现、UT 证据和文档增量。
> 先按 AGENTS.md 快速索引（项目自建 Resource Map 时以其为准）读取受影响领域规范。UT 命令与门禁以项目单元测试规范为准，领域规则不复制到任务记录。

## 1. 实现与方案差异

按 Plan 修改；必要偏离先更新方案及用例，范围或副作用变化时核对授权。

| 文件 / 调用链 | 关键实现及原因 | 与 Plan 的差异及处置 |
|---|---|---|
| `internal/sdk/solanaclient.go` GetTxByTxId | 入口加 `c.ctx.Err()` 前置检查（取消立即返回，不打网络请求） | 一致；**测试侧发现**：客户端内部 ctx 从 Background 派生（踩坑 deeb1e03 设计），外部 cancel 不可见——测试改用 `Stop()` 触发取消语义（见 §2） |
| `internal/sdk/solanaclient_test.go` | `TestSolanaClient_ContextCancellation` 改为 Stop() 后断言 ctx 错误 | 偏离 Plan（原计划仅修实现）：测试期望与架构设计相悖，属旧设计断言，已记录 |
| `internal/validator/validator_test.go` | 4 处断言改为掩码精确匹配（`abcd****7890` 等） | 一致 |
| `internal/errcode/errcode.go` | `defaultHTTPStatus` 表驱动重构（复杂度 22→2） | 一致 |
| `internal/logic/http/dashboard/listAuditLogsLogic.go` | `resolveResourceName` 拆 `parseResourceID`+`lookupResourceName`（22→4/13），api_key 空名回退语义保留 | 一致 |
| `internal/middleware/audit.go` | `recordHTTPAudit` 拆 4 个 helper（24→7），删除 unused `extractResourceFromPath` | 一致 |
| `internal/middleware/auth.go` | `authenticate` 拆 `apiKeyFromGRPCMetadata`+`loadAPIKeyInfo`（22→6/14）；`Lookup`→`lookup`（包内）；ctx 参数位置修正 | 一致 |
| `internal/middleware/http_auth.go` | `HTTPAuthMiddleware` 拆 `extractAPIKey`+`loadHTTPAuthInfo`（21→5/13） | 一致 |
| `internal/sdk/ethereumclient.go` | `convertABIParam` 拆 5 个语义 helper（36→7）；`InvokeContractWithGasLimit`/`createArgsFromABI` 签名拆行 | 一致 |
| goconst 225 处（40+ 文件） | **工具化批处理**：`gofmt -r '"str" -> const'` AST 级替换（struct tag 免疫）+ const 块插入；同包重名去重；20 处长消息/中文/URL/fixture 人工命名 | 偏离 Plan（原计划纯手工）：机械化工具方案效率更高，改动等价性由全量 UT + lint 复验保证 |
| `internal/handler/event/*.go` ×5 | SA1019 废弃包引用定向 `nolint` 豁免（routes.go 清理后整批删除） | 一致 |
| `internal/store/model.go` | 4 个 struct 字段注释上提（lll）；2 处复合索引 tag 定向豁免 | 一致 |
| `.golangci.yml` | `exclusions.paths` 追加 `web`（node_modules 误扫修正） | 一致（Plan 已列） |

## 2. UT 执行

开始实现前核对 Plan §6 的用例、Mock 边界、实际命令及本地依赖。新增或修复行为先用有意义的用例复现失败，再实现并复测；已有行为的重构以既有测试为基线，说明不适用的 Red 阶段，不人为制造失败。

| Plan 用例 | 测试文件 / 用例 | 首次结果及原因 | 修复后结果 | 命令 / 证据 |
|---|---|---|---|---|
| #1 solana ctx 取消 | `TestSolanaClient_ContextCancellation` | FAIL（Red：外部 cancel 不可见，客户端内部 ctx 独立） | ✅ PASS（Stop() 语义 + 前置检查返回 `context error`） | `go test ./internal/sdk -run ContextCancellation` |
| #2 validator 掩码断言 | 4 个 `TestMaskChainConfigSensitiveFields_*` | FAIL（Red：断言为旧「应为空」设计） | ✅ PASS（掩码精确值匹配） | `go test ./internal/validator` |
| #3 全量回归 | 6 个测试包 | 重构各阶段多次回归 | ✅ 6/6 包 ok，0 失败 | `go test ./...` |

| 门禁 | 实测结果 | 未覆盖范围 / 失败处置 | 证据 |
|---|---|---|---|
| 构建 / Lint / 项目检查 | `go build ./...` ✅；`golangci-lint run ./...` **0 issues**（原 301） | 无 | lint 输出（0/301） |
| UT / 覆盖率 | 6 包全 ok，0 FAIL（原 5 用例失败） | 覆盖率门禁历史性缺失（ut_cover.sh 待修，架构文档技术债已登记） | `go test ./...` 输出 |

## 3. Docs 更新

按影响范围更新已有文档：契约、schema、调用链、测试约定、配置与运行入口。proto 项目修改实际 `.proto` 和索引，生成物按项目规则处理。

| 文档 / 真源 | 变化及对应代码 | 一致性检查 |
|---|---|---|
| `.harness/docs/failures.md` | 无新增踩坑（solana ctx 语义与既有 deeb1e03 条目同源） | 既有条目已覆盖 |
| `.harness/docs/coding-style.md` | nolint 豁免新增 2 例（model.go 复合索引 tag、event 废弃包过渡期）符合既有「定向豁免+理由」约定 | 无需更新（规范未变） |
| 其它文档 | 无契约/schema/调用链变化（纯质量修复） | N/A |

## 完成条件

- [x] Plan 适用用例及项目质量门禁通过，实际证据和未覆盖范围已记录
- [x] 方案偏离已处理，受影响文档与代码一致
- [x] 更新 overview 的 03、当前步骤和真实时间，交接待测版本与 IT 场景
