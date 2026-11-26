# 部署与运行规范

> 状态：生效（部分环境为 TODO） | 维护者：jackz | 最后更新：2026-11-23（sop.init 填实）

## 范围

覆盖测试环境、预发、生产三套环境的发布、配置、运行规范。

## 1. 环境矩阵

| 环境 | 用途 | 域名 | 数据 | 访问方式 | 审批 |
|------|------|------|------|----------|------|
| dev | 个人开发 | `localhost`（8085/8080/6061） | 本地 PG/Redis（Docker） | 本机 | 无 |
| test | QA 测试 | K8s 集群内 Service | 测试库（postgresql:5432） | 内网 / port-forward | 无 |
| pre | 预发验证 | <!-- TODO(sop.init) --> | 仿生产（脱敏） | 受控 | 团队 leader |
| prod | 生产 | <!-- TODO(sop.init) --> | 真实 | 严格审批 | 部门 + 安全 |

## 2. 制品

- 构建产物：**Docker 镜像**（两阶段构建：golang:1.22.7 builder → ubuntu:24.04 运行层）
- 制品仓库：`192.168.1.2:5000`（私有 registry，Makefile `IMAGE`）
- 命名规则：双 tag——`chain-interactive-service:${VERSION}`（如 v1.1.0）+ `chain-interactive-service-${REV}:${VERSION}`（REV=git short HEAD）
- 必须注入：`build_time`、`git_commit`、`version`（通过 `make build` 的 LDFLAGS 自动注入 `internal/version.go`）
- 构建命令：`make build-docker`（含 chainmaker SDK vendor 化 + modvendor C 库复制）
- 保留策略：
  - 生产：≥**1 年**
  - 预发：≥**90 天**
  - 测试：≥**30 天**

## 3. 配置管理

### 配置中心
- 工具：无配置中心——本地 `etc/chaininteractive.yaml` + K8s ConfigMap（helm `values.yaml` 渲染）
- 注意：本项目**单一 yaml**（未分环境命名文件）；环境差异靠 ConfigMap / 值覆盖

### 敏感配置
- **【禁止】** 把真实凭证、密钥、Token 提交到代码库
- **【必须】** 通过环境变量或 K8s Secret 注入（Redis 密码 / DB DSN）
- **【必须】** Redis 密码、DB DSN 不明文写在 yaml；使用环境变量注入
- 配置变更必须走流程：审核 → 灰度 → 全量 → 留存变更记录
- `etc/chainmaker_sdk_config.yml` / `etc/client.key`：链 SDK 兜底配置与密钥，Dockerfile COPY 进镜像；**新提交严禁再含密钥**

### 环境隔离
- 不同环境使用**不同的**密钥、Token、第三方账号
- **禁止**测试环境连生产 DB / 第三方真实账号

## 4. 发布流程

```
1. 预检：CI 全绿 + Code Review 通过 + 关联单号
2. 测试环境：make build-docker 构建镜像 → helm 升级（deploy/helm/）
3. 预发环境：人工审批 + 冒烟用例
4. 生产环境（金丝雀阶梯）：
   5% → 等待 15 分钟 → 25% → 等待 30 分钟 → 50% → 等待 30 分钟 → 100%
   每阶梯监控核心指标，告警则自动暂停（HPA 2-10 副本配合 PDB minAvailable 1）
5. 回滚：触发条件 + 命令（helm rollback）+ 数据兼容性确认
```

### 灰度阶梯阈值
- 错误率 > **1%** 暂停
- P99 延迟 > 1s 暂停（链上操作除外，以链出块时间为基线）
- 业务核心指标偏离基线 > **5%** 暂停

### 回滚
- **必须**保留上一稳定版本制品至少 30 天
- 回滚命令必须演练（至少季度一次）
- 数据库迁移**禁止**直接物理回滚（用 forward fix）

## 5. 健康检查

| 探针 | 端点 | 通过条件 | 间隔 |
|------|------|----------|------|
| Liveness | `GET :6061/healthz`（go-zero DevServer） | HTTP 200 | helm 探针配置 |
| Readiness | `GET :6061/healthz` | HTTP 200 | helm 探针配置 |
| Metrics | `GET :6061/metrics` | Prometheus 抓取 | 抓取周期 |

> 本项目 Liveness/Readiness 均基于 `/healthz`（helm 模板定义）；Dockerfile 无 EXPOSE/HEALTHCHECK，端口与探针由 K8s 层管理。

- 启动时间预算：≤**60s**
- 健康检查**禁止**调用第三方耗时接口

## 6. 监控告警

### 指标平台
- 指标：**Prometheus 抓取 `:6061/metrics`**（go-zero 内建）+ Grafana 展示
- 日志：文件日志 `logs/`（按天切割保留 7 天）；K8s 场景收集方式<!-- TODO(sop.init) 按运维实际填写 -->
- 链路追踪：OpenTelemetry → Jaeger（`Telemetry` 配置节，**当前 Disabled**，启用即生效）
- APM：无

### 必备指标
- 错误率（HTTP 5xx / 业务错误码）
- P50 / P95 / P99 接口延迟
- QPS / 调用量
- 依赖可用性（DB、Redis、下游服务）
- 资源占用（CPU、内存、磁盘、连接数）
- 业务核心指标：合约调用量（Invoke/Query 分类）、事件订阅数、配额拦截次数、账单金额

### 告警阈值（默认值，按业务调整）

| 指标 | 警告 | 严重 |
|------|------|------|
| 错误率 | > 0.5% 持续 5min | > 1% 持续 1min |
| P99 延迟 | > 800ms 持续 5min | > 1s 持续 1min |
| CPU 使用率 | > 70% 持续 10min | > 85% 持续 5min |
| 内存使用率 | > 75% 持续 10min | > 85% 持续 5min |
| 磁盘使用率 | > 80% | > 90% |
| 连接池饱和度 | > 70% | > 85% |
| 慢查询数 | > 10/min | > 50/min |

### 告警通道
- <!-- TODO(sop.init) 按团队实际填写（企业微信/短信/电话） -->
- 严重告警必须电话；重复 3 次未响应升级到 leader

## 7. 容量规划

- 单实例容量基线：<!-- TODO(sop.init) 压测后填写 -->；资源 requests 100m/128Mi，limits 1000m/512Mi（values.yaml）
- 弹性策略：**HPA 2-10 副本**（CPU>70% / Mem>80%），PDB minAvailable 1
- 大促/活动：必须**提前压测**（至少 N+1 倍流量），扩容到位再上线
- 多副本注意：限流为进程内（总 QPS 放大 N 倍）、订阅可能重复（Leader 选举未接线，见架构文档技术债）

## 8. 日志规范

- 生产日志级别：**info**（debug 仅限故障排查临时开启）
- **【禁止】** 日志输出：密码、Token、密钥、完整身份证、银行卡、指针值
- **【必须】** 关键路径日志：请求入口（含 trace_id）、外部调用（成功/失败）、错误堆栈
- 日志格式：JSON 结构化，便于检索
- 日志留存：**7 天**（go-zero Log KeepDays 配置；合规场景按要求延长）
- 日志体积：单实例日均 ≤10GB（超出考虑采样或下沉）

## 9. 应急预案

| 故障类型 | 现象 | 应对动作 |
|----------|------|----------|
| 依赖 X 不可用 | 告警 + 错误率上升 | 开关降级 + 联系上游 + 切备 |
| 流量突增 | CPU/QPS 飙高 | 限流 + 扩容 + 排查异常调用方 |
| 单机异常 | 单机告警 | 流量摘除 + 重启 + 复盘 |
| DB 慢/锁表 | 慢查询暴增 | kill 慢查询 + 切只读 / 主备切换 |
| 缓存雪崩 | DB 压力骤增 | 限流 + 缓存重建 + 热点 key 复查 |

每个预案必须**演练**（季度一次），并记录到 [failures.md](failures.md)。

## 10. 模拟上下游服务

> 本地开发与测试环境中，上下游依赖服务可能不可用或不稳定。本节定义如何模拟（Mock/Stub）这些依赖，确保服务可独立启动和调试。

### 依赖服务清单

| # | 依赖服务 | 协议 | 模拟方式 | 端口 | 说明 |
|---|----------|------|----------|------|------|
| 1 | PostgreSQL | TCP | Docker 真实容器 | 5432 | `docker run postgres:14`（见 development.md） |
| 2 | Redis | TCP | Docker 真实容器 | 6379 | `docker run redis:7` |
| 3 | 区块链节点 | 各链 RPC/WS | **不可模拟**——连测试网 | — | Ethereum 测试网 / ChainMaker 测试链 / Solana devnet |

> 链节点无法本地模拟：依赖 `TenantChainConfig` 指向测试网端点（test-connection 预检）。

### 启动方式

#### 方式一：拉起依赖服务（完整联调）

- **启动命令**：`docker run -d --name cis-pg ...` / `docker run -d --name cis-redis ...`（见 development.md §必装依赖）
- **就绪检查**：`curl :6061/healthz`

#### 方式二：不启动服务，注入 Mock 响应（轻量开发）

当不需要真实拉起上下游服务时，可通过以下方式避免调用报错：

| # | 方式 | 本项目采用 | 适用场景 |
|---|------|-----------|----------|
| 1 | 环境变量开关（跳过/走桩逻辑） | 否 | — |
| 2 | 接口层 Stub 实现（DI 注入空实现） | 部分：`mockgen` 可生成 `ChainPlugin`/`Repository` mock（UT 用） | 单元测试 |
| 3 | 配置降级（调用失败返回默认值） | 否 | — |
| 4 | 本地请求拦截（中间件/代理返回预设响应） | 否 | — |
| 5 | hosts / DNS 映射（指向共享 Mock 平台） | 否 | — |

> **原则**：服务必须能在上下游缺失时正常启动。核心依赖走 Mock，非核心依赖走降级。

### Mock 数据管理

- 目录：`mock/`（mockgen 产物，`make gen-mock`）
- 数据格式：Go 源码（接口 mock）
- **【推荐】** Mock 产物纳入版本管理，团队共享

### 调用约定

| # | 规范 | 说明 |
|---|------|------|
| 1 | **【必须】** 通过配置切换真实/模拟地址 | 禁止硬编码模拟地址到业务代码 |
| 2 | **【必须】** Mock 服务地址使用环境变量 | 如 `CIS_IT_HTTP` |
| 3 | **【必须】** Mock 数据覆盖正常 + 异常场景 | 至少包含：成功、参数错误、超时、服务不可用 |
| 4 | **【禁止】** Mock 服务暴露到公网 | 仅限本地/内网使用 |
| 5 | **【禁止】** 生产配置指向 Mock 服务 | CI 中增加检查，防止误配 |

### 本地开发启动流程

```
1. 拉取代码并安装依赖（go mod tidy）
2. 启动依赖服务：docker run postgres:14 / redis:7
3. 确认依赖就绪：docker ps + healthz
4. 启动本服务（etc/chaininteractive.yaml dev 配置）：make start-service
5. 验证调用：curl :6061/healthz + 注册租户走一遍合约调用
```

## 11. 禁止项

- ❌ 直接登录生产服务器手动改文件 / 跑脚本
- ❌ 跨过审批流程直接发布
- ❌ 在生产执行未演练的 SQL（DDL / DML）
- ❌ 测试环境用真实生产数据（必须脱敏）
- ❌ 关闭生产监控 / 告警（除维护窗口）
- ❌ 上线时间窗口外发布生产（除非紧急 hotfix，且事后补流程）

## 参考

- 上下游依赖：[relationship.md](../relationship.md)
- 失败案例：[failures.md](../failures.md)
- 编码与安全：[coding-style.md](../coding-style.md)
