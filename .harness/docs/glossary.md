# 业务术语表

> 状态：生效 | 维护者：jackz | 最后更新：2026-11-23（sop.init 填实）

## 范围

**只收录本项目私有的、非公知的术语。** 行业通用概念（API、SDK、Docker、gRPC、ORM 等）和编程语言标准术语不在本表范围——AI 和工程师都已经知道这些。

本表的价值在于：一个熟练的高级工程师首次接触本项目时，哪些词他**无法靠经验推断**其含义。

## 使用规则

- 代码/文档中首次使用本表中的术语时，用全称或括号注明缩写含义
- 新创缩写必须先在本表注册，禁止在代码中直接使用未登记的私有缩写
- 发现同义词混用时，在「易混术语对照」段统一约定，后续只用一种
- AI 遇到不认识的项目特有词汇时，应主动查阅本表或追问

## 术语表

| 术语 | 英文/缩写 | 别名（旧称） | 定义（本项目语境） | 出处/代码路径 |
|------|-----------|-------------|-------------------|--------------|
| 租户 | Tenant | — | BaaS 商业化隔离单元：拥有独立用户、API Key、链配置、配额与账单 | `internal/store/model.go` |
| 租户链配置 | TenantChainConfig | — | 租户自带的一条链接入配置（含三链专属字段 + 节点列表），运行时动态加载替代 etc 下 SDK yaml | `internal/store/model.go` |
| 租户链节点 | TenantChainNode | — | ChainMaker 专用的节点地址/TLS 配置（挂在 TenantChainConfig 下） | `internal/store/model.go` |
| 租户合约配置 | TenantContractConfig | — | 租户在某条链上的合约（名称/地址/ABI/是否订阅），CallContract 的解析目标 | `internal/store/model.go` |
| 套餐 | Plan | — | 租户计费档位：free / developer / enterprise | `Tenant.Plan` |
| 配额 | Quota | — | 租户级调用限额（日/月限额 + QPS 限流 + 超额策略 OveragePolicy） | `internal/store/model.go`、`internal/billing/` |
| 超额策略 | OveragePolicy | — | 超过配额后的处置（拒绝/放行计费） | `Quota.OveragePolicy` |
| 调用日志 | CallLog | — | 每次合约调用的记录（链/方法/状态/耗时），账单与用量统计的数据源 | `internal/store/model.go` |
| 账单 | Bill | — | 按租户周期生成的费用汇总（period_start/end、total_calls、amount） | `internal/billing/` |
| 实时费用 | RealtimeCost | — | dashboard 实时计费视图（未结算用量折算） | `internal/logic/http/dashboard/` |
| 事件订阅 | Subscribe / EnableSubscribe | — | 为租户合约开启链上事件订阅，事件经 Redis Stream 推送给 gRPC 流消费端 | `internal/sdk/` |
| 订阅断点 | LatestBlockHeight | — | Redis 中保存的已消费区块高度，服务重启后续传 | `internal/sdk/*client.go` |
| 链类型 | ChainType | — | Ethereum / Chainmaker / Solana（proto 枚举；BTC 预留） | `proto/chaininteractive.proto` |
| 方法类型 | MethodType | Invoke/Query | Invoke=写链交易（计费）；Query=只读查询 | `proto/chaininteractive.proto` |
| 插件 | ChainPlugin | — | 统一链适配接口，三链各有一个内置实现；新链接入=实现该接口+注册工厂 | `internal/sdk/interface.go`、`internal/plugin/` |
| 测试连接 | VerifyConnection | test-connection | 用租户链配置实测链连通性（曾因校验缺失误报通过，见 failures `fb0fc824`） | `internal/validator/` |
| 脱敏 | Mask / KeyMasked | — | 私钥前 4 后 4、API Key 前 8 后 4 的日志/列表掩码 | `internal/validator/validator.go` |

> 收录标准：高级工程师首次看到这个词，能否不查资料就理解？不能 → 收录。能 → 不收录。

## 项目缩写速查

| 缩写 | 全称 | 说明 |
|------|------|------|
| BaaS | Blockchain as a Service | 本项目的产品形态：多链统一交互平台 |
| DSN | Data Source Name | `DatabaseConf.DSN`，DB 连接串（含凭证，注意脱敏） |

## 易混术语对照

| 术语 A | 术语 B | 区别 | 本项目统一用 |
|--------|--------|------|-------------|
| CallContract | SendTransaction | 语义相同，SendTransaction 为旧称（已改名） | CallContract |
| 链配置 ChainConfig | 合约配置 ContractConfig | 前者是链接入层（节点/SDK），后者挂在其下的合约 | 各自全称 |
| Quota（配额） | RateLimit（限流） | Quota=日/月总量限额（DB）；RateLimit=瞬时 QPS（进程内滑动窗口） | 各自全称 |

## 弃用术语

| 旧术语 | 弃用日期 | 替换为 | 原因 |
|--------|----------|--------|------|
| SendTransaction | 2026-06 前后 | CallContract | 接口语义改名，屏蔽链差异 |

## 不收录清单（提醒）

以下类型的术语**不要**添加到本表：
- 行业标准：API、SDK、CI/CD、Docker、K8s、HTTP、gRPC、REST、MQ...
- 语言标准：goroutine、channel、Promise、async/await、interface...
- 通用数据库：SQL、NoSQL、索引、事务、migration...
- 通用设计模式：MVC、单例、工厂、观察者...
- **公开协议/规范定义的术语**：DID、VC、VP、OAuth2、OIDC、JWT、JSON-LD、W3C 标准中定义的任何概念...
- **通用基础设施**：KMS、HSM、SSO、DAL、DAO、CDN、LB、MQ...
- **名字自解释的外部服务**：看全称就能理解的机构或服务（如 CFCA、GLEIF）...
- **标准角色名**：Issuer/Holder/Verifier 等在公开规范中有定义的角色...
- 任何一个高级工程师不用查就能理解的词
- 任何搜索引擎搜一下就能找到准确定义的词
