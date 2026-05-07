# Chain Interactive Service 架构说明文档

**[English](architecture_en.md)** | **中文**

---

## 1. 项目概述

Chain Interactive Service 是一个通用区块链交互服务平台（BaaS - Blockchain as a Service），提供统一的 gRPC 和 RESTful API 接口与多种区块链（Ethereum、ChainMaker、Solana）进行交互，屏蔽底层链差异，使上层业务无需关心链的具体实现细节。

项目已从单纯的区块链中间件演进为支持**多租户**、**计费配额**、**插件化架构**、**安全审计**的商业化 BaaS 平台。

---

## 2. 系统架构

### 2.1 整体架构图

```mermaid
graph TB
    subgraph "客户端层"
        WebDashboard[Web 管理控制台]
        SDKClient[SDK 客户端<br/>Go/JS/Python/Java]
        GRPCClient[gRPC 客户端]
    end

    subgraph "接入层"
        HTTPGateway[HTTP API Gateway<br/>:8080]
        GRPCServer[gRPC Server<br/>:9000]
    end

    subgraph "中间件层"
        Auth[认证中间件<br/>API Key 验证]
        RBAC[权限控制<br/>RBAC]
        RateLimit[限流中间件<br/>滑动窗口]
        Quota[配额检查<br/>日/月限额]
        Audit[审计日志<br/>自动记录]
        Anomaly[异常检测<br/>自动封禁]
    end

    subgraph "业务逻辑层"
        TenantSvc[租户管理服务]
        BillingSvc[计费服务]
        Logic[合约调用逻辑]
    end

    subgraph "SDK 管理层"
        TenantSDKMgr[租户级 SDK 管理器]
        PluginRegistry[插件注册中心]
    end

    subgraph "链插件层"
        EthPlugin[Ethereum 插件]
        CMPlugin[ChainMaker 插件]
        SolPlugin[Solana 插件]
        MorePlugin[更多链插件...]
    end

    subgraph "基础设施层"
        PostgreSQL[(PostgreSQL<br/>租户/账单/审计)]
        Redis[(Redis<br/>事件订阅/缓存)]
        K8s[Kubernetes<br/>弹性伸缩]
    end

    WebDashboard --> HTTPGateway
    SDKClient --> HTTPGateway
    GRPCClient --> GRPCServer

    HTTPGateway --> Auth
    GRPCServer --> Auth
    Auth --> RBAC
    RBAC --> RateLimit
    RateLimit --> Quota
    Quota --> Audit
    Audit --> Anomaly

    Anomaly --> TenantSvc
    Anomaly --> BillingSvc
    Anomaly --> Logic

    Logic --> TenantSDKMgr
    TenantSDKMgr --> PluginRegistry

    PluginRegistry --> EthPlugin
    PluginRegistry --> CMPlugin
    PluginRegistry --> SolPlugin
    PluginRegistry --> MorePlugin

    TenantSvc --> PostgreSQL
    BillingSvc --> PostgreSQL
    Audit --> PostgreSQL
    EthPlugin --> Redis
    CMPlugin --> Redis
    SolPlugin --> Redis
    K8s -.-> GRPCServer
    K8s -.-> HTTPGateway
```

### 2.2 请求处理流程

```mermaid
sequenceDiagram
    participant C as 客户端
    participant GW as API Gateway
    participant Auth as 认证中间件
    participant RBAC as 权限控制
    participant RL as 限流器
    participant Q as 配额检查
    participant L as 业务逻辑
    participant SDK as SDK 管理器
    participant Chain as 区块链节点

    C->>GW: HTTP/gRPC 请求 (API Key)
    GW->>Auth: 验证 API Key
    Auth->>Auth: 检查 Key 有效性/IP 白名单
    Auth->>RBAC: 传递租户上下文
    RBAC->>RBAC: 检查角色权限
    RBAC->>RL: 通过权限检查
    RL->>RL: 滑动窗口限流
    RL->>Q: 通过限流
    Q->>Q: 检查日/月配额
    Q->>L: 通过配额检查
    L->>SDK: 获取链客户端
    SDK->>Chain: 发送交易/查询
    Chain-->>SDK: 返回结果
    SDK-->>L: 返回结果
    L-->>C: 响应结果
    
    Note over Q,L: 异步记录审计日志和用量
```

---

## 3. 功能架构

### 3.1 功能模块总览

```mermaid
mindmap
  root((Chain Interactive<br/>Service))
    核心功能
      合约调用 (Invoke/Query)
      交易查询
      事件订阅
      链信息查询
    多租户体系
      租户管理
      子账号管理
      API Key 生命周期
      RBAC 权限控制
    计费系统
      配额管理
      用量统计
      账单生成
      超额策略
    安全体系
      API Key 认证
      IP 白名单
      审计日志
      异常检测与封禁
      敏感数据脱敏
    插件化架构
      插件注册中心
      链插件接口
      内置插件适配器
      动态加载
    API 接入
      gRPC 接口
      RESTful HTTP API
      管理后台 API
    部署运维
      Helm Chart
      HPA 自动伸缩
      Leader 选举
      健康检查
```

### 3.2 核心功能模块

| 模块 | 描述 | 关键文件 |
|------|------|----------|
| **合约调用** | 统一接口调用多链合约（Invoke/Query） | `internal/logic/callcontractlogic.go` |
| **交易查询** | 根据交易 ID 查询交易状态和详情 | `internal/logic/gettxbytxidlogic.go` |
| **事件订阅** | 订阅链上合约事件，推送到 Redis | `internal/sdk/*.go` |
| **链信息查询** | 查询可用链和合约配置 | `internal/logic/getavailablechainandcontractnameslogic.go` |

### 3.3 商业化功能模块

| 模块 | 描述 | 关键文件 |
|------|------|----------|
| **多租户管理** | 租户创建/禁用/启用、子账号、API Key 管理 | `internal/tenant/service.go` |
| **认证鉴权** | API Key 认证 + RBAC 权限控制 | `internal/middleware/auth.go`, `rbac.go` |
| **计费配额** | 配额检查、用量记录、账单生成 | `internal/billing/service.go` |
| **限流** | 基于滑动窗口的 QPS 限流 | `internal/middleware/ratelimit.go` |
| **审计日志** | 自动记录所有操作的审计日志 | `internal/middleware/audit.go` |
| **异常检测** | 失败频率监控、自动封禁 | `internal/middleware/anomaly.go` |
| **管理后台 API** | 仪表盘、日志查询、账单查询 | `internal/gateway/admin_handlers.go` |

---

## 4. 目录结构

```
chain-interactive-service/
├── chaininteractive.go              # 服务主入口
├── chaininteractive/                # goctl 生成的业务逻辑层
├── internal/
│   ├── config/
│   │   └── config.go               # 配置定义与校验
│   ├── logic/                       # gRPC 业务逻辑
│   │   ├── callcontractlogic.go     # 合约调用逻辑
│   │   ├── gettxbytxidlogic.go      # 交易查询逻辑
│   │   └── getavailablechainandcontractnameslogic.go
│   ├── sdk/                         # 链 SDK 客户端
│   │   ├── interface.go             # 统一链接口定义
│   │   ├── helper.go               # SDK 客户端管理与订阅调度
│   │   ├── ethereumclient.go       # 以太坊客户端实现
│   │   ├── chainmakerclient.go     # 长安链客户端实现
│   │   ├── solanaclient.go         # Solana 客户端实现
│   │   ├── solana_codec.go         # Solana Borsh 编解码
│   │   └── tenant_sdk_manager.go   # 租户级 SDK 管理器
│   ├── store/                       # 数据持久化层
│   │   ├── model.go                # 数据模型定义
│   │   ├── db.go                   # 数据库连接
│   │   └── repository.go          # Repository 接口与实现
│   ├── gateway/                     # HTTP API Gateway
│   │   ├── server.go              # Gateway 服务启动
│   │   ├── routes.go              # 路由注册
│   │   ├── handlers.go            # 核心 API Handler
│   │   └── admin_handlers.go      # 管理后台 API Handler
│   ├── middleware/                  # 中间件
│   │   ├── auth.go                # gRPC 认证拦截器
│   │   ├── http_auth.go           # HTTP 认证中间件
│   │   ├── rbac.go                # RBAC 权限控制
│   │   ├── ratelimit.go           # 限流中间件
│   │   ├── quota.go               # 配额检查中间件
│   │   ├── audit.go               # 审计日志中间件
│   │   └── anomaly.go             # 异常检测与封禁
│   ├── billing/                     # 计费系统
│   │   └── service.go             # 计费服务实现
│   ├── tenant/                      # 租户管理
│   │   └── service.go             # 租户服务实现
│   ├── plugin/                      # 插件化架构
│   │   ├── registry.go            # 插件注册中心
│   │   └── builtin.go             # 内置链插件适配器
│   ├── deploy/                      # 部署相关
│   │   └── leader_election.go     # 分布式 Leader 选举
│   ├── server/                      # gRPC 服务注册
│   ├── svc/                         # 服务上下文
│   │   └── servicecontext.go      # ServiceContext 依赖注入
│   └── code/                        # 响应码定义
├── proto/                           # Protobuf 定义
│   └── chaininteractive.proto
├── pb/                              # 生成的 Protobuf Go 代码
├── deploy/                          # 部署配置
│   └── helm/                       # Helm Chart
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
├── docker/                          # Docker 构建
├── etc/                             # 配置文件
├── scripts/                         # 脚本工具
└── doc/                             # 文档
```

---

## 5. 数据模型

### 5.1 ER 关系图

```mermaid
erDiagram
    Tenant ||--o{ User : "拥有"
    Tenant ||--o{ APIKey : "拥有"
    Tenant ||--o{ TenantChainConfig : "配置"
    Tenant ||--o{ CallLog : "产生"
    Tenant ||--o{ Bill : "生成"
    Tenant ||--|| Quota : "分配"
    Tenant ||--o{ AuditLog : "记录"
    
    User ||--o{ APIKey : "创建"
    TenantChainConfig ||--o{ TenantContractConfig : "包含"

    Tenant {
        uint id PK
        string name UK
        string email UK
        string status
        string plan
    }

    User {
        uint id PK
        uint tenant_id FK
        string username UK
        string role
        string status
    }

    APIKey {
        uint id PK
        uint tenant_id FK
        uint user_id FK
        string key UK
        string permissions
        string ip_whitelist
        time expires_at
    }

    TenantChainConfig {
        uint id PK
        uint tenant_id FK
        string chain_name
        string chain_type
        bool enable
        text sdk_conf
    }

    TenantContractConfig {
        uint id PK
        uint tenant_id FK
        uint chain_config_id FK
        string contract_name
        string contract_addr
        text abi_json
    }

    CallLog {
        uint id PK
        uint tenant_id FK
        string chain_name
        string method
        string status
        int64 duration
    }

    Bill {
        uint id PK
        uint tenant_id FK
        time period_start
        time period_end
        uint64 total_calls
        float64 amount
    }

    Quota {
        uint id PK
        uint tenant_id FK
        uint64 monthly_limit
        uint64 daily_limit
        int rate_limit
    }

    AuditLog {
        uint id PK
        uint tenant_id FK
        uint user_id FK
        string action
        string resource
        text detail
    }
```

---

## 6. 插件化架构

### 6.1 插件接口

所有链实现都必须实现 `ChainPlugin` 接口：

```go
type ChainPlugin interface {
    Name() string                    // 插件名称
    ChainType() string               // 链类型
    Version() string                 // 插件版本
    Init(ctx, conf) error            // 初始化
    HealthCheck(ctx) error           // 健康检查
    CallContract(...)                // 调用合约
    GetTxByTxId(txId) (...)          // 查询交易
    SubscribeContractEvent(...)      // 订阅事件
    Stop() error                     // 停止释放资源
}
```

### 6.2 插件注册流程

```mermaid
sequenceDiagram
    participant App as 应用启动
    participant Reg as 插件注册中心
    participant Factory as 插件工厂
    participant Plugin as 链插件实例

    App->>Reg: RegisterBuiltinPlugins()
    Reg->>Reg: 注册 Ethereum 工厂
    Reg->>Reg: 注册 ChainMaker 工厂
    Reg->>Reg: 注册 Solana 工厂

    App->>Reg: CreatePlugin(name, chainType, conf)
    Reg->>Factory: factory()
    Factory-->>Reg: 新插件实例
    Reg->>Plugin: Init(ctx, conf)
    Plugin-->>Reg: 初始化完成
    Reg-->>App: 返回插件实例
```

---

## 7. 部署架构

### 7.1 Kubernetes 部署

```mermaid
graph TB
    subgraph K8sCluster["Kubernetes Cluster"]
        subgraph IngressLayer["Ingress"]
            IngressNode[Nginx Ingress]
        end

        subgraph ServiceLayer["Service"]
            SVC[ClusterIP Service<br/>gRPC:9000 / HTTP:8080]
        end

        subgraph DeploymentLayer["Deployment (HPA: 2~10)"]
            Pod1[Pod 1<br/>chain-interactive]
            Pod2[Pod 2<br/>chain-interactive]
            PodN[Pod N<br/>chain-interactive]
        end

        subgraph ConfigMapLayer["ConfigMap"]
            CM[chaininteractive.yaml]
        end

        subgraph DepsLayer["Dependencies"]
            PG[(PostgreSQL)]
            RD[(Redis)]
        end

        HPA[HPA<br/>CPU>70% / Mem>80%]
        PDB[PDB<br/>minAvailable: 1]
    end

    IngressNode --> SVC
    SVC --> Pod1
    SVC --> Pod2
    SVC --> PodN
    CM -.-> Pod1
    CM -.-> Pod2
    CM -.-> PodN
    Pod1 --> PG
    Pod1 --> RD
    HPA -.-> Pod1
    PDB -.-> Pod1
```

### 7.2 Leader 选举

多实例环境下，事件订阅任务通过 Redis 分布式锁实现 Leader 选举，确保每个订阅任务只有一个实例执行：

```mermaid
sequenceDiagram
    participant P1 as Pod 1
    participant P2 as Pod 2
    participant Redis as Redis

    P1->>Redis: SETNX leader_key pod1 (TTL=15s)
    Redis-->>P1: OK (获取锁成功)
    P2->>Redis: SETNX leader_key pod2 (TTL=15s)
    Redis-->>P2: FAIL (锁已存在)

    Note over P1: 成为 Leader，执行订阅任务

    loop 每 5 秒续约
        P1->>Redis: EXPIRE leader_key 15s
        Redis-->>P1: OK
    end

    Note over P1: Pod 1 宕机
    Note over Redis: 15 秒后锁自动过期

    P2->>Redis: SETNX leader_key pod2 (TTL=15s)
    Redis-->>P2: OK (获取锁成功)
    Note over P2: 成为新 Leader
```

---

## 8. 安全架构

### 8.1 安全防护层次

| 层次 | 机制 | 说明 |
|------|------|------|
| **接入层** | API Key 认证 | 每个请求必须携带有效的 API Key |
| **网络层** | IP 白名单 | 可限制 API Key 只能从指定 IP 访问 |
| **权限层** | RBAC | 基于角色的访问控制（admin/developer/readonly） |
| **流量层** | 限流 + 配额 | 防止滥用，保护系统稳定性 |
| **检测层** | 异常检测 | 短时间大量失败请求自动封禁 |
| **审计层** | 审计日志 | 所有操作自动记录，支持事后追溯 |
| **数据层** | 敏感数据脱敏 | 日志中自动 mask 私钥、密码等敏感字段 |

### 8.2 认证流程

```mermaid
flowchart TD
    A[请求到达] --> B{携带 API Key?}
    B -->|否| C[返回 401]
    B -->|是| D{Key 有效?}
    D -->|否| E[返回 401]
    D -->|是| F{Key 过期?}
    F -->|是| G[返回 401]
    F -->|否| H{IP 白名单检查}
    H -->|不通过| I[返回 403]
    H -->|通过| J{租户状态正常?}
    J -->|否| K[返回 403]
    J -->|是| L{异常检测-是否被封禁?}
    L -->|是| M[返回 403]
    L -->|否| N[通过认证，继续处理]
```

---

## 9. 技术栈

| 类别 | 技术 | 版本 |
|------|------|------|
| **语言** | Go | 1.22+ |
| **框架** | go-zero | v1.6.2 |
| **通信** | gRPC + Protobuf | - |
| **HTTP** | go-zero/rest | - |
| **数据库** | PostgreSQL / MySQL | - |
| **ORM** | GORM | v1.25+ |
| **缓存** | Redis | - |
| **链 SDK** | go-ethereum | v1.14.11 |
| **链 SDK** | chainmaker-sdk-go | v2.3.8 |
| **链 SDK** | solana-go | v1.8.3 |
| **监控** | Prometheus + OpenTelemetry | - |
| **部署** | Kubernetes + Helm | - |
| **容器** | Docker | - |

---

## 10. API 接口总览

### 10.1 gRPC 接口

| 方法 | 描述 |
|------|------|
| `CallContract` | 调用/查询链上合约 |
| `GetTxByTxId` | 根据交易 ID 查询交易 |
| `GetAvailableChainAndContractNames` | 获取可用链和合约列表 |

### 10.2 RESTful HTTP API

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/contract/call` | 调用合约 |
| GET | `/api/v1/transaction/:txId` | 查询交易 |
| GET | `/api/v1/chains` | 获取可用链列表 |
| POST | `/api/v1/tenants` | 创建租户 |
| GET | `/api/v1/tenants` | 租户列表 |
| POST | `/api/v1/tenants/:id/disable` | 禁用租户 |
| POST | `/api/v1/tenants/:id/enable` | 启用租户 |
| POST | `/api/v1/api-keys` | 创建 API Key |
| GET | `/api/v1/api-keys` | API Key 列表 |
| POST | `/api/v1/chain-configs` | 创建链配置 |
| GET | `/api/v1/chain-configs` | 链配置列表 |
| PUT | `/api/v1/chain-configs/:id` | 更新链配置 |
| DELETE | `/api/v1/chain-configs/:id` | 删除链配置 |
| GET | `/api/v1/users` | 用户列表 |
| GET | `/api/v1/dashboard/overview` | 仪表盘概览 |
| GET | `/api/v1/dashboard/call-logs` | 调用日志 |
| GET | `/api/v1/dashboard/usage-stats` | 用量统计 |
| GET | `/api/v1/dashboard/bills` | 账单列表 |
| GET | `/api/v1/dashboard/audit-logs` | 审计日志 |
