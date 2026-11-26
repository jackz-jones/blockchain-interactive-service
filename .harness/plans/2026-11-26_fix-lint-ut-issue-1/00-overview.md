# [2026-11-26] 修复 lint 债务与失败 UT（issue #1）
> **本文件是本任务的单一真相源（Single Source of Truth）**：任务元信息、进度、当前步骤、关键决策全部在这里。
> 会话恢复时，先读本文件定位当前步骤，再按需加载对应阶段文件。
>
> ⚠️ 本项目**不维护**全局 `.harness/plan.md`——跨任务查看请列 `.harness/plans/` 目录。
> ⚠️ Meta 中的 `分支` 字段是上下文恢复时定位任务的唯一依据，**必须**与 `git branch --show-current` 的输出完全一致。

---

## Meta

| 项 | 值 |
|----|----|
| 分支 | `feature/issue-1-fix-lint-ut` |
| GitHub Issue | `Closes #1`（修复 optimize_sop1 分支 lint 和 UT 问题） |
| 摘要 | 清零 301 个 lint 问题（goconst/lll/staticcheck/gocyclo/revive/gofmt/unused）+ 修复 5 个失败测试（validator 4 + solana 1），恢复 push 门禁 |
| 状态 | 🔵 进行中 |
| 创建日期 | 2026-11-26 |
| 负责人 | jackz |
| 预期完成 | 2026-11-27 |
| 开发模式 | 独立开发 |
| 测试环境 | — |
| 小需求模式 | ⬜（>10 行，goconst 224 处机械改动 + 测试修复） |
| 预估代码改动行数 | （Plan 阶段填入） |

---

## Progress

<!-- 本文件结构 / 字段定义 / Progress / 时间记录 SOP 规则见
     .harness/plans/_template/00-overview.md；本任务文件精简不重复。
     init_harness.sh 只刷新 _template/。 -->

- [x] 01. Clarify    → [01-clarify.md](./01-clarify.md) (背景/目标/4 项决策全部确认)
- [x] 02. Plan       → [02-plan.md](./02-plan.md) (四类修复策略/UT 以既有 Red 为基线/大需求判定)
- [x] 03. Implement  → [03-implement.md](./03-implement.md) (lint 清零/UT 全绿/gocyclo 6 处重构/goconst 225 处常量化)
- [x] 04. IT         → [04-it.md](./04-it.md) (本地门禁链路：lint 0 + 6 包 ok；push 门禁随 05 验证)
- [ ] 05. Review & Commit → [05-review-commit.md](./05-review-commit.md)

---

## 当前步骤

> 恢复会话时，优先读取此处指向的阶段文件。

- **步骤**：⏳ 05. Review & Commit
- **文件**：[05-review-commit.md](./05-review-commit.md)
- **上次更新**：2026-11-26 10:42:59

---

## 时间记录

<!-- 本文件结构 / 字段定义 / Progress / 时间记录 SOP 规则见
     .harness/plans/_template/00-overview.md；本任务文件精简不重复。
     init_harness.sh 只刷新 _template/。 -->

| # | 步骤 | 开始时间 | 结束时间 | 耗时 | 备注 |
|---|------|---------|---------|------|------|
| 01 | Clarify    | 2026-11-26 10:06:51 | 2026-11-26 10:11:04 | 4m13s |  |
| 02 | Plan       | 2026-11-26 10:11:04 | 2026-11-26 10:13:41 | 2m37s |  |
| 03 | Implement  | 2026-11-26 10:13:41 | 2026-11-26 10:42:09 | 28m28s | lint 301→0 + UT 5→0 失败 + 6 包全绿 |
| 04 | IT         | 2026-11-26 10:42:09 | 2026-11-26 10:42:59 | 50s | 以本地门禁链路代替（无部署变更，见 04-it.md §1） |
| 05 | Review & Commit | 2026-11-26 10:42:59 |  |  |  |
| 03 | Implement  |  |  |  |  |
| 04 | IT         |  |  |  |  |
| 05 | Review & Commit |  |  |  |  |

---

## 关键决策备忘

> **跨阶段共享的关键上下文**。仅记录影响后续步骤的决策，避免恢复时还要翻阅历史阶段文件。
> 填写建议：接口命名最终选型、DataType 字段设计、兼容性约束、与其它模块的约定、跳过的步骤及原因等。

- validator 失败测试**改断言**（实现为掩码，正确行为）；solana 失败**修实现**（`GetTxByTxId` 入口加 `ctx.Err()` 前置检查）——2026-11-26 用户确认
- goconst 224 处**全量一次清**（机械提取常量，改后全量 UT 回归）——2026-11-26 用户确认
- PR 策略：**堆叠 PR**（先合 `optimize_sop1` → master 的 SOP 基建 PR，本任务 PR base=`optimize_sop1`）——2026-11-26 用户确认
- lint 规范阈值**不改**（`.golangci.yml` v2 配置已 amend 进 `52b837c`，本任务只清代码不动配置）

---

## 风险速览

> 任务级风险登记。高严重风险可按需在 `.harness/docs/` 下单独归档，避免跨任务共享文件造成合并冲突。

| # | 风险 | 严重度 | 缓解 |
|---|------|-------|------|
| 1 | 224 处 goconst 机械改动可能引入行为回归 | 🟡 中 | 只提取常量不改逻辑；全量 UT 回归 |
| 2 | gocyclo 7 处降复杂度涉及小重构 | 🟡 中 | 优先提前返回/抽函数；风险大处单独确认 |

## 文件索引

| 文件 | 产物 |
|------|------|
| [00-overview.md](./00-overview.md) | 任务总览（本文件） |
| [01-clarify.md](./01-clarify.md) | 需求澄清：背景、目标、范围、待确认问题 |
| [02-plan.md](./02-plan.md) | 方案设计：改动文件、调用链、数据模型、IT 用例 |
| [03-implement.md](./03-implement.md) | 实现、UT 结果与覆盖率、文档更新 |
| [04-it.md](./04-it.md) | 部署与集成测试、用例证据、环境收尾 |
| [05-review-commit.md](./05-review-commit.md) | 评审处置、GitHub Issue/PR、提交与 amend |

---

## 完成条件

> 本文件承载时间记录这个强制项，因此自身也要有可校验出口；五个阶段各自完成时逐条核对，不满足项如实标注原因。

- [ ] Meta 的 `分支` 与 `git branch --show-current` 完全一致，`状态` 反映真实进度
- [ ] Progress 的 01–05 每行有且仅有一行，完成项已原地替换为 `[x]` 并带结果摘要
- [ ] 时间记录 01–05 每行要么有阶段边界实测的开始 / 结束时间与耗时，要么备注列明确标注「漏记」；**不存在无依据的时间戳**
- [ ] 「当前步骤」指向的阶段文件与实际所在阶段一致，「上次更新」为最近一次真实修改时间
- [ ] 「关键决策备忘」已登记影响后续阶段的选择（接口选型、兼容约束、跳过步骤及原因）
- [ ] 「风险速览」与 01 §3 / 02 §8 的结论没有互相矛盾
- [ ] 「文件索引」中所有链接目标真实存在
