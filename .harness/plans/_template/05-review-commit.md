# 05. Review & Commit

> 【本地化】本项目为 GitHub 开源项目：以 GitHub Issue + Pull Request 替代 TAPD / 工蜂 MR 流程。
>   ⚠️ sop.init 升级会刷新本模板回 TAPD 版，升级后需重做本节本地化。
> 输入：最终 diff、03/04 的验证与环境回执；输出：自审处置、提交准备和已授权交付结果。
> 自审依据项目 code-review 规范及 `.harness/review.md`；提交格式和 Git 操作依据 `.harness/commit.md`。

## 1. 提交前自审

按项目 Check ID 审查最终 diff，不复制检查清单。每项记录通过、问题或 N/A 及依据；没有 Check ID 时引用审查条款。发现行为变化回 03 验证，并复测受影响的 04 场景。

| Check ID / 条款 | 结论 | 文件位置及证据 | 问题处置 / 风险接受 |
|---|---|---|---|
|  |  |  |  |

自审人与人工审批人分开。提交前只记录真实自审结论；PR 创建后的人工审批以 GitHub PR review 为准，不预填 Reviewer 批准或签字。

例：自审通过但 PR 尚未审批，交付说明写“自审通过，人工审批待完成”。

## 2. 环境与 GitHub Issue

核对 04 的原租约释放回执，已释放不重复申请或释放；未收尾时保留负责人和待办。

确认任务关联的 GitHub Issue；已明确提供的直接核验（issue 编号、标题、状态），不重复询问。没有关联 issue 时询问：AI 代创建（GitHub issue，用 02 的详细方案 + 03 的 UT / 04 的 IT 结果填充内容）、用户提供、或按授权跳过并记录原因。GitHub 不可达时报告缺口，不猜字段或绕过流程。

提交信息以 `Closes #<issue>` / `#<issue>` 关联 issue（hook 校验，见 `.harness/commit.md`）；PR 创建后在 PR 描述中再次引用，合并即自动关闭 issue。

| 提交前准备 | 已核验结果 / 依据 | 阻塞或跳过原因 |
|---|---|---|
| 环境收尾 |  |  |
| GitHub Issue 编号、方案和自测材料 |  |  |
| 提交前状态 / 下游待办 |  |  |

## 3. 提交准备

提交信息按 `.harness/commit.md` 和实际 hook 定稿，关联格式按项目要求。

```text
<本次实际 commit message>
```

提交前检查暂存范围，只包含本任务代码、测试和必要文档。overview 记录“交付待核验”和准备时间，保留 05 未完成；push、PR、issue 状态尚未发生时不得标为完成。

### ChainWeaver 部署所需的提前提交

部署依赖远端版本时，在已有授权内先建立本任务提交并推送；此时不结束 05、不冻结任务文档。后续修复与最终文档通过 amend 汇入同一提交，改写远端历史使用 `--force-with-lease`。

## 4. 已授权交付与复核

1. commit 或 amend；push 后核对远端 SHA。
2. 在授权范围内创建 / 复用 GitHub Pull Request（`gh pr create` 或网页），PR 描述引用 issue（`Closes #N`）并附 02/03/04 摘要；push 成功不代表 PR 存在，创建后回读核验。
3. CI / Review 导致变更时，修复、更新受影响证据并重跑必要检查，按项目一个 PR 一个 commit 规则 amend。暂存前逐项检查，不使用无范围的 `git add -A`。
4. 核查 PR 状态（CI 检查、review approvals、merge）；未 merge 时报告具体待办与负责人，不宣布端到端完成。无 PR 的已批准交付按实际范围验收。PR 合并后确认关联 issue 已自动关闭（`Closes #N`）。

## 完成条件与恢复

提交材料准备完成可随最终提交保存；提交后的 SHA、PR、审批和 issue 状态以 GitHub 为准，在交付回复中汇报，不为补登元数据额外创建提交。
恢复“交付待核验”任务时先查询远端和 GitHub（`gh pr list` / `gh issue view`）：达到授权终点且验证、环境、PR 均满足适用条件时报告已完成；否则从真实剩余项继续。PR 合入后的新增改动建立新任务，不改写已合入历史。

- [ ] 自审逐条有结论，`BLOCKED` / 未验证项已写明原因，未把未执行写成通过
- [ ] 暂存范围逐项列出并核对 diff，无无范围 `git add -A`，未混入其他任务改动
- [ ] GitHub Issue 编号与状态、PR 状态已按真实系统核验，未发生的结果不预填
- [ ] `00-overview.md` 的 05 行时间记录已按阶段边界实测值填写（未记则标注「漏记」，禁止回填），状态改为真实值，「当前步骤」与「上次更新」同步
