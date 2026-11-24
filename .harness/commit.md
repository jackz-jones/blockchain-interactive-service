# Commit 规范

> AI 与人类共同遵守的代码提交规范，是 Commit 阶段的权威来源；`AGENTS.md` 只做索引引用。
> 若项目已有独立 commit 规范（commitlint / CI 校验 / 内部规范），直接在下方「项目自定义」区替换或补充，并保持本节总纲一致。

---

## 1. Commit Message 格式

```
<type>(<scope>): <subject>

<body 含 GitHub issue 引用，如 Closes #123>
```

### type 取值

| type | 场景 |
|------|------|
| `feat` | 新功能 |
| `fix` | 问题修复 |
| `refactor` | 重构（不新增功能不修 bug） |
| `docs` | 文档变更 |
| `test` | 测试相关 |
| `chore` | 构建 / 工具 / 杂项 |
| `perf` | 性能优化 |

### scope / subject

- `scope`：影响模块（如 `api`、`db`、`cli`），小改动可省略。
- `subject`：祈使句、小写开头、≤ 50 字符、不以句号结尾。

### GitHub Issue 关联（本项目必填）

- 引用格式：`#123`（裸引用，仅关联）或 `Closes #123`（PR 合并时自动关闭 issue；关键词 close/closes/closed/fix/fixes/fixed/resolve/resolves/resolved）。
- 位置：commit message 的 title 或 body 任意位置（body 末尾独立一行最常见）。
- 校验：`scripts/commit_msg_check.sh`（commit-msg hook 强制，正则 `#[0-9]+`）。
- 跳过：无关联 issue 时 `DISABLE_ISSUE_FOOTER=1 git commit ...`（需在 `00-overview.md` 关键决策备忘记原因）。

---

## 2. 执行与状态

执行顺序、提前部署提交和 GitHub Issue/PR 衔接见当前任务 `05-review-commit.md`。已经明确授权的普通步骤连续执行；范围或副作用发生变化时再确认，授权不自动扩展到 push、PR、合入或部署。

提交前记录真实准备状态；push、PR、审批及 issue 状态按 GitHub 真实系统核验，未发生时不标完成。确认 05 到达本次授权终点后汇报，任务记录不为补登 Git 元数据追加提交。

## 3. 一个 PR 一个 commit

- 最终交付只有本任务一个提交，修复和必要文档更正使用 amend；行为变化时同步方案与测试证据，不能为冻结文档而保留失真记录。
- amend 后推送使用 `git push --force-with-lease`。每次暂存明确列出本任务文件并核对 diff，避免混入用户或其他任务改动。
- PR 合入后不改写历史；新工作另建任务。commit、push、PR、合入及部署按各自授权范围执行。

---

## 项目自定义

> **本项目（GitHub 开源项目）已固化的规则**：

- 关联系统：**GitHub Issue**（`https://github.com/jackz-jones/blockchain-interactive-service/issues`）
- 关联格式：`Closes #<issue>`（推荐，合并自动关闭）/ `#<issue>`（裸引用）——**必填**，hook 强制（见 §1）
- 交付载体：**GitHub Pull Request**（`gh pr create` 或网页创建；PR 描述同样引用 `Closes #N`）
- 合入规则：一个 PR 一个 commit，修正走 amend + `git push --force-with-lease`（见 §3）
- 本项目无 TAPD / 工蜂 / Jira 等内部系统依赖；sop.init 升级会刷回通用模板文案，保留本节「项目自定义」即可恢复语义。
