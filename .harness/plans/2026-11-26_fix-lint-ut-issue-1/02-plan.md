# 02. Plan

> **目的**：把 Clarify 的结论转化为可落地的技术方案。
> **输入**：`01-clarify.md` 的目标与范围（4 项决策已确认）
> **输出**：改动清单、调用链、UT 用例、风险与回滚
> **TDD 模式**：本任务以**既有失败用例为 Red**（solana ctx / validator 断言）；其余为机械重构，以既有测试为基线，跳过 Red 的理由见 §6。

---

## 1. 方案概述

分四类修复 301 个 lint 问题与 5 个失败测试：① 修复类（solana `GetTxByTxId` 入口加 `ctx.Err()` 前置检查、validator 测试断言对齐掩码实现）；② 机械类（goconst 224 处提取包级常量、lll 32 处拆行、QF1008 23 处嵌入字段简化、gofmt 1 处格式化）；③ 判断类（SA1019 5 处弃用 API 换替代、SA9003 空分支、QF1012 context、gocyclo 7 处提前返回/抽函数、revive 4 处）；④ 死代码（unused 2 处删除）。**不改** `.golangci.yml` 阈值；修复后全量 lint + UT 回归；IT 以本地门禁代替（不涉及部署变更）。

## 2. 改动文件清单

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `internal/sdk/solanaclient.go` | 修改 | ① `GetTxByTxId` 入口加 `ctx.Err()` 前置检查 ② goconst 常量提取（"finalized"/"confirmed" 等确认级别、签名前缀）③ lll 拆行 ④ gocyclo 降复杂度 |
| `internal/sdk/solanaclient_test.go` | 修改 | goconst/lll 机械修复（不改断言语义，ctx 用例即本任务 Red） |
| `internal/sdk/solana_codec.go` | 修改 | goconst 12 处（Borsh 编解码中的字段名/程序地址常量）+ lll |
| `internal/sdk/solana_process.go` | 修改 | goconst / staticcheck 机械修复 |
| `internal/sdk/types.go` | 修改 | goconst 26 处（链名/币种/单地址常量）+ QF1008 |
| `internal/sdk/ethereumclient.go` | 修改 | gocyclo 1 处 + goconst + QF1008（订阅循环降复杂度：提前返回） |
| `internal/sdk/tenant_sdk_manager.go` | 修改 | revive/gocyclo 1 处 |
| `internal/sdk/helper.go` | 修改 | goconst（订阅 Redis key 前缀常量） |
| `internal/validator/validator.go` | 修改 | goconst / QF1008（**不改掩码行为**） |
| `internal/validator/validator_test.go` | 修改 | 4 处断言改为掩码匹配（`expectMasked`：`len>0 && strings.Contains(prefix/后缀)`），Red→Green |
| `internal/middleware/auth.go` / `http_auth.go` / `audit.go` / `apikey_cache.go` | 修改 | goconst + gocyclo + QF1008（鉴权头/事件名常量提取；提前返回） |
| `internal/middleware/rbac.go` | 修改 | 删除 unused `extractResourceFromPath`（确认无引用后删） |
| `internal/logic/http/contractconfig/updateContractConfigLogic.go` | 修改 | revive 1 处（函数长度/导出注释） |
| `internal/logic/http/dashboard/listAuditLogsLogic.go` | 修改 | 删除 unused `buildContractConfs`（确认无引用后删） |
| `internal/logic/http/`（若干 logic 文件） | 修改 | goconst/lll 机械修复（错误文案/字段名常量） |
| `internal/errcode/errcode.go` | 修改 | goconst 2 处（错误消息常量） |
| `internal/billing/service.go` | 修改 | goconst + gocyclo（账单状态常量；计费区间判断提前返回） |
| `internal/code/respCode.go` | 修改 | goconst（响应消息常量） |
| `internal/service/config_resolver.go` | 修改 | goconst 13 处（配置解析缓存 key 常量） |
| `.golangci.yml` | 修改 | **仅一处**：exclusions.paths 追加 `web`（node_modules 被误扫，属配置遗漏修正而非阈值放宽） |
| `internal/billing/service_test.go` | 修改 | gocyclo/revive 测试代码机械修复 |

> goconst 224 处分布广，上表列主力文件；Implement 时按 lint 报告逐文件清零，发现新文件在 03 记录补充。

## 3. 影响范围

| 维度 | 影响 |
|------|------|
| 接口 | **无**（gRPC/HTTP 契约、proto/.api 不动） |
| 模块 | internal/sdk、middleware、logic、validator、billing、errcode 内部重构 |
| DB schema | 无 |
| 配置 | 无（etc/*.yaml 不动） |
| 协议兼容 | 无（对外行为不变；solana ctx 取消语义修正属错误路径行为改进） |
| 上下游服务 | 无 |

## 4. 调用链

```
gRPC/HTTP 入口
  → middleware（auth/rbac/audit）  [修改：常量提取+提前返回，行为不变]
    → logic
      → sdk.ChainPlugin
        → solana.GetTxByTxId        [修改：入口 ctx.Err() 前置检查（新增节点行为：
          → solana RPC                 ctx 已取消立即返回 Canceled，不再发网络请求）]
```

> 其余链路节点不变；重构均为节点内部实现调整。

## 5. 数据结构变更

### 5.1 内部 DataType / Schema

无新增字段；仅提取既有字面量为包级常量（不导出，`camelCase` 私有常量，命名遵循 `coding-style.md`）。

### 5.2 DB 表结构

无变更。

### 5.3 协议 / 接口契约

无变更（`proto/`、`api/` 不动，`make gen-code` 不触发）。

## 6. UT 用例设计（TDD 必填，先于 Implement）

| # | 被测对象（函数 / 类 / 模块路径） | 测试文件（计划） | 类型 | 输入 | 期望输出 / 行为 | Mock 边界 |
|---|----------------------------------|------------------|------|------|----------------|-----------|
| 1 | `internal/sdk.SolanaClient.GetTxByTxId` | `internal/sdk/solanaclient_test.go`（既有 `TestSolanaClient_ContextCancellation`） | 异常（ctx 取消） | 已取消的 ctx + 合法 txId | 返回错误**含 `context`**（修复后立即返回，不打网络请求） | mock HTTP server（既有） |
| 2 | `internal/validator.MaskChainConfigSensitiveFields` | `internal/validator/validator_test.go`（既有 4 用例改断言） | 边界 | 含私钥/API Key 的配置 | 私钥字段=掩码（前4+`****`+后4），**非空**；断言改匹配掩码格式 | 无 |
| 3 | 既有全量测试 | `go test ./...` | 回归 | — | 全绿（重构零行为回归的基线证据） | 既有 mock |

> **跳过 Red 的理由**（模板允许：已有行为的纯重构以既有测试为基线）：#1 即本任务 Red（当前失败中），修复转绿；#2 为断言对齐（旧断言已失真）；机械重构不人为制造失败用例。
> Implement 阶段若 SA1019 替换 API / gocyclo 重构改动行为，必须回本节补充用例。

## 7. IT 用例设计

| # | 场景 | 类型 | 前置条件 | 执行步骤 | 预期结果 |
|---|------|------|---------|---------|---------|
| 1 | 本地门禁链路（等价 IT） | 正向 | 依赖就绪（无外部依赖） | `golangci-lint run ./...` → `go test ./...` → `git push`（不带 skip 变量） | lint 0 问题 / 测试全绿 / push 门禁通过 |

> 本任务无部署与运行时接口变更（solana ctx 修正属错误路径），按项目 IT 规范以本地门禁代替真实环境链路；04 阶段记录原因。

## 8. 风险与兜底

| 风险 | 触发条件 | 影响 | 缓解 | 回滚方案 |
|------|---------|------|------|---------|
| goconst 批量改动引入回归 | 常量提取时误改字符串值 | 运行时行为变化 | 只提取不改值；每类文件改完跑该包 UT | git revert 单 commit |
| SA1019 替换弃用 API 行为差异 | 新旧 API 语义不完全一致 | solana/链交互行为变化 | 逐处检查替代 API 文档；改动小且单测覆盖 | revert |
| 删除 unused 函数影响后续规划 | 函数实为预留功能 | 后续需重写 | PR 中列出删除清单（`extractResourceFromPath` / `buildContractConfs`），issue #1 评论留痕 | revert |
| gocyclo 重构改变执行顺序 | 提前返回引入短路 | 边界条件行为变化 | 只做等价变换（guard clause）；相关包 UT + code review 自审 | revert |

## 9. 小需求模式判定

判定结论：

```text
[大需求] ｜ 触发维度：<范围> ｜ 理由：涉及 ~15 文件 300+ 行批量改动 + 删除死代码 + 弃用 API 替换，需完整 UT 回归链验证
```

| 排除项核对 | 是否触犯 | 依据 |
|-----------|---------|-------------------------------------|
| 改变已上线接口的请求 / 响应契约 | 否 | 契约文件不动 |
| DB schema / 迁移变更 | 否 | 无 |
| 核心业务逻辑 | **是** | `internal/sdk` ctx 前置检查与弃用 API 替换触及链交互核心路径 |
| 并发核心 | 否 | 不动并发结构 |
| 跨项目 / 跨模块 / 跨服务 / 复杂异步链路 | 否 | 单仓库内 |
| 不可逆 / 高风险操作 | 否 | 全部可 revert |

> 「核心业务逻辑」排除项触犯 → 标准模式（大需求），Meta `小需求模式` 保持 ⬜。

## 10. 工时估算

| 阶段 | 工时 | 备注 |
|------|------|------|
| Implement | 2h | 含测试修复（Red→Green）+ 机械清零 |
| UT | 1h | 全量回归 + 门禁验证 |
| Deploy + IT | 0 | 以本地门禁代替（§7） |
| Docs + Review | 0.5h | failures.md 沉淀 + 自审 |
| **预估代码改动行数** | **~350** | 不含测试；仅作汇报信息 |

---

## 完成条件

- [x] 改动清单逐文件、调用链、风险与回滚、§6 UT 用例、§7 IT 用例均已定稿，关键偏离和决策已同步 `00-overview.md`
- [x] §9 小需求模式判定已输出结论（大需求，Meta 保持 ⬜）；判定所用排除项逐条有依据
- [ ] `00-overview.md` 的 02 行时间记录已按阶段边界实测值填写，并同步「当前步骤」与「上次更新」
- [ ] 完成后可直接进入 03 Implement
