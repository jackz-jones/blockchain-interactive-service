# 01. Clarify

> **目的**：把模糊想法澄清为可执行需求。本阶段聚焦 **背景 / 目标 / 风险点** 三件事，务必与用户充分对齐。
> **推荐辅助**：使用 `skill: clarify` 澄清需求；AI 切换为「产品经理」角色完成 Discovery + Challenge 双阶段对话。若 skill 不可用，按下方「决策框架」手工推进。
> **输入**：用户原始描述 / GitHub Issue 链接
> **输出**：本文件填写完整 + `00-overview.md` 关键决策备忘登记要点

---

## 1. 背景 (Context)

- GitHub Issue #1（`修复 optimize_sop1 分支 lint 和 UT 问题`）：`optimize_sop1` 分支推送被 pre-push 门禁拦截——golangci-lint 报 **301 个问题**、`go test` **2 个包 5 个用例失败**。
- 起因：`.golangci.yml` 迁移 v2（适配本机 golangci-lint 2.12.2，commit `52b837c`）后 lint 首次真实生效，暴露分支存量债务；此前旧配置加载失败，lint 实际从未跑过。
- 不做会怎样：push 门禁持续拦截（每次需 `SKIP_PRE_PUSH=1` 应急绕过），CI 无法恢复，代码质量口径失效。

## 2. 目标 (Goal)

**主要目标**：清零 lint 问题 + 修复失败测试，恢复 push 门禁正常通道。

**成功指标（可验证）**：
| 指标 | 当前值 | 目标值 | 验证方式 |
|------|-------|-------|---------|
| golangci-lint 问题数 | 301 | **0** | `golangci-lint run ./...` |
| go test 失败用例 | 5（validator 4 + solana 1） | **0** | `go test ./...` |
| push 门禁 | 需 SKIP_PRE_PUSH=1 绕过 | 正常通过 | `git push` 不带 skip 变量 |

**lint 问题分布**（2026-11-26 实测）：goconst 224 / lll 32 / staticcheck 31 / gocyclo 7 / revive 4 / gofmt 1 / unused 2

## 3. 风险点

| # | 风险 | 严重度 | 缓解 / 兜底 |
|---|------|-------|------------|
| 1 | goconst 224 处机械改动量大，批量改可能引入行为回归 | 🟡 中 | 只提取常量不改逻辑；改动后跑全量 UT 回归 |
| 2 | validator 测试修复方向（改断言 vs 改实现）涉及安全语义 | 🟠 高 | 实现为踩坑 `98bffb6c` 后的正确掩码形态（与 KeyMasked 风格一致）；倾向改测试断言——待用户确认 |
| 3 | 修 unused 死代码（删函数）可能影响后续规划中的功能 | 🟡 中 | 仅删除确认无引用的 2 个函数，PR 说明列出 |
| 4 | gocyclo 7 处降复杂度涉及重构，可能超机械改动范畴 | 🟡 中 | 优先小幅重构（提前返回/抽函数）；风险大时与用户确认替代方案 |

## 4. 待确认问题 (Open Questions)

| # | 问题 | 结论 | 决策人 |
|---|------|------|-------|
| 1 | validator 4 个失败测试：实现为掩码（前4后4），测试断言为「应为空」——按**改测试断言**处理？ | ✅ **改测试断言**（匹配掩码格式；实现为踩坑后正确行为，2026-11-26 用户确认） | 用户 |
| 2 | solana `TestSolanaClient_ContextCancellation` 失败：入口无 ctx 前置检查——按**修实现**处理？ | ✅ **修实现**（`GetTxByTxId` 入口加 `ctx.Err()` 检查，取消立即返回 `context.Canceled`，2026-11-26 用户确认） | 用户 |
| 3 | goconst 224 处全量提取常量还是分批？ | ✅ **全量一次清**（约 300+ 行机械改动，改后全量 UT 回归，2026-11-26 用户确认） | 用户 |
| 4 | PR base：`optimize_sop1`（堆叠 PR）还是 `master`（一次带 2 commit）？ | ✅ **堆叠 PR**（先合 optimize_sop1 → master 的 SOP 基建 PR，本任务 PR base=optimize_sop1，2026-11-26 用户确认） | 用户 |

## 5. 关联 (References)

- GitHub Issue：https://github.com/jackz-jones/blockchain-interactive-service/issues/1
- 相关接口 / 设计文档 / ADR：`.harness/docs/coding-style.md`（lint 规范）、`.harness/docs/failures.md`（踩坑 98bffb6c / deeb1e03）
- 上游 / 下游依赖：无（纯代码质量任务，不影响外部接口）

---

## 决策框架

1. **5W1H 自检**：Why / What / Who / Where / When / How 每项能否一句话回答？
2. **INVEST**：需求是否满足 Independent、Negotiable、Valuable、Estimable、Small、Testable？

## 反例

❌ **目标含糊** → "`DescribeInstances` P99 从 800ms 降到 200ms，通过 benchmark 脚本在 dev 环境 5000 QPS 下验证。"
❌ **风险点无缓解** → 写明"风险 + 严重度 + 缓解/兜底"，避免"已知有风险，但暂未想到方案"。

## 完成条件

- [ ] 背景与可量化目标已写明，风险各有缓解，Open Questions 全部有结论且关键决策已同步 `00-overview.md`
- [ ] `00-overview.md` 的 01 行时间记录已按阶段边界实测值填写（未记则标注「漏记」，禁止回填），并同步「当前步骤」与「上次更新」
- [ ] 完成后可直接进入 02 Plan
