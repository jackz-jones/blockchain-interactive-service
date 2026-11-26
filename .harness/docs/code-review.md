# 代码 Review 规范

> 状态：生效 | 维护者：jackz | 最后更新：2026-11-23（sop.init 填实）

## 范围

所有合入 `master` 的 PR/MR 必须经过 Review，AI 生成代码同样适用。

## 1. 提交方约束

- ❌ 直接推 `master`（必须走特性分支 + PR）
- ❌ PR > **500** 行未拆分（按模块/层级拆）
- ❌ PR 未关联 GitHub Issue（`Closes #N` / `#N`）
- ❌ AI 代码未经理解就提交（提交者必须能回答："这段代码做了什么？为什么这样写？"）
- ❌ 删除已有测试（除非有明确理由并在 PR 描述中说明）
- ❌ 引入新依赖但 commit message 未说明理由
- ❌ 提交 `.env` / `*.key` / `*.pem` / `credentials.json` / 编译产物
- ✅ PR 描述包含：背景、改动点、影响面、测试方式、回滚方案

## 2. 提交方自查清单（pre-commit）

- [ ] `make build` 通过
- [ ] `make lint` 通过（golangci-lint）
- [ ] `make ut` 通过，覆盖率不下降（目标 ≥60%；当前基线见 `make ut` 输出）
- [ ] 新增/修改逻辑有对应测试（TDD：先红后绿）
- [ ] 无 `.env` / `*.key` / `vendor/` / `node_modules/` / `bin/` / `logs/` / `cover.out` 等
- [ ] commit message 符合 `<type>(<scope>): <subject>` + GitHub issue 引用（`Closes #N` / `#N`，hook `scripts/commit_msg_check.sh` 校验）
- [ ] 契约变更已同步 `proto/` / `.api` 并 `make gen-code`（routes.go 与 .api 双核对）
- [ ] 我**完全理解**这段代码做了什么（不是 AI 生成后未读直接提交）
- [ ] 关联 GitHub Issue 编号

## 3. Reviewer 检查清单

### P0（必查 / 阻断合入）

#### 业务正确性
- [ ] 改动符合需求文档/Spec，无范围蔓延
- [ ] 边界条件：空值、null、超长输入、并发、重试幂等
- [ ] 错误处理完整，无吞错（`_ = err` 必须有理由注释）

#### 安全
- [ ] 无 SQL 注入（参数化查询）
- [ ] 无命令注入（结构化 API 调用）
- [ ] 无路径穿越（`..` 校验）
- [ ] 无敏感信息硬编码（密钥、AK/SK、密码、内部 IP）
- [ ] 日志脱敏（身份证、手机号、Token、密码不打印）
- [ ] 鉴权/越权检查到位（操作权限、数据权限）
- [ ] 输入校验完整（白名单 + validator）
- [ ] 加密算法符合规范（无 MD5/SHA1/DES/RC4）

#### 资源安全
- [ ] 资源释放配对（defer / with / try-with-resources）
- [ ] 无 goroutine/线程/Promise 泄漏
- [ ] Context 正确传递并响应取消
- [ ] 无内存泄漏（订阅、监听、缓存）

#### 测试
- [ ] 新代码有测试覆盖（包含正常 + 边界 + 异常）
- [ ] 断言有效（非 message 字符串断言）
- [ ] 集成测试用真实 DB（**禁止** mock DB）
- [ ] 修改逻辑必须有对应单元测试调整

### P1（推荐 / 强烈建议）

- [ ] 命名一致、注释解释 Why
- [ ] 函数 ≤80 行、嵌套 ≤4 层、行长 ≤120 列、圈复杂度 ≤20（与 `.golangci.yml` 对齐）
- [ ] 复杂度可控，无明显坏味道（重复、过长参数、上帝类）
- [ ] 性能：N+1、不必要的全表扫描、内存浪费
- [ ] 可观测性：关键路径有 `logx.WithContext` 日志、关键指标埋点
- [ ] go-zero 框架规范：Logic 通过 `svcCtx` 注入依赖、context 透传、`pb/`/`internal/types` 生成代码无手改
- [ ] 链交互专项：SDK 内部 ctx 用 `context.Background()` 派生（红线 5）；gRPC 拦截器 Unary/Stream 同步；链上操作挂 Quota+Audit 中间件（红线 4）
- [ ] 多实例一致性：新增订阅/定时任务是否考虑多副本重复执行（Leader 选举未接线）

### AI 幻觉专项

> 因 AI 生成代码占比上升，必须额外检查：

- [ ] 引用的方法/字段/包**真实存在**（防 AI 编造 API）
- [ ] 第三方库版本/用法与 `go.mod` / `package.json` / `requirements.txt` 一致
- [ ] 没有"看起来对但实际不存在"的导入
- [ ] 业务逻辑符合实际数据流（不是 AI 想象的流程）
- [ ] 关键魔术值（超时、阈值、限流参数）有依据，非凭空填的"合理数字"
- [ ] 错误码引用真实存在的常量

### 可选（按场景）

- [ ] 国际化：硬编码文案应否提取
- [ ] 兼容性：API 字段变更、数据库迁移影响
- [ ] 文档：README / ARCHITECTURE / OpenAPI 是否同步更新

## 4. Review 回复礼仪

- 区分严重程度：
  - `must-fix`（阻断）
  - `should-fix`（强烈建议）
  - `nice-to-have`（可选改进）
  - `nit`（鸡毛蒜皮，不阻塞合入）
  - `question`（澄清，提交者必须回应）
- 给出**具体修改建议**而非"这写得不好"
- 提交者：每条评论必须回应（采纳 / 解释 / 暂缓 + 跟进 issue）
- 评论无回应、无修改不能合入

## 5. 合入门槛

- ≥**1** 名 Reviewer 批准（关键模块 ≥**2**，跨模块改动需对应模块负责人）
- CI 全绿（build / lint / ut；integration-test 落地后追加）
- 所有 `must-fix` 已解决
- 无未回复的 `question`
- 无 `Security Critical` 告警

## 6. AI 辅助 Review

- 可使用 `/code-review` 或类似 Skill 获得初步审查报告
- AI 报告作为**参考输入**，**不替代**人工 Review
- 人工 Review 必须由非提交者进行（本人不能 review 自己的 PR）

## 7. 紧急合入流程（hotfix）

- 仅限：生产事故、安全漏洞、强制下线
- 必须：
  - 至少 1 名 Reviewer 同步评审（语音/电话）
  - 24 小时内补 PR 描述与回归测试
  - 事后复盘记录到 [failures.md](failures.md)

## 参考

- 编码规范：[coding-style.md](coding-style.md)
- 测试规范：[unittest/unittest.md](unittest/unittest.md)
- API 规范：[api-standards.md](apis/api-standards.md)
- 失败案例：[failures.md](failures.md)
