# 使用指南（中文）

本文档提供 Chain Interactive Service 的详细使用说明，包括 gRPC 客户端集成、HTTP API 参考、各链特定配置和高级功能。

## 目录

- [服务启动](#服务启动)
- [gRPC 客户端集成](#grpc-客户端集成)
- [API 参考](#api-参考)
  - [CallContract](#callcontract)
  - [GetTxByTxId](#gettxbytxid)
  - [GetAvailableChainAndContractNames](#getavailablechainandcontractnames)
- [各链使用说明](#各链使用说明)
  - [Ethereum](#ethereum)
  - [ChainMaker](#chainmaker)
  - [Solana](#solana)
- [事件订阅](#事件订阅)
- [合约调用同步/异步模式](#合约调用同步异步模式)
- [多租户 HTTP API](#多租户-http-api)
- [Web 管理控制台](#web-管理控制台)
- [TLS 配置](#tls-配置)
- [监控](#监控)
- [错误处理](#错误处理)
- [最佳实践](#最佳实践)

---

## 服务启动

### 基本启动

```bash
# 编译并使用默认配置运行
make build
./chain-interactive-service -f etc/chaininteractive.yaml
```

### 版本检查

```bash
./chain-interactive-service version
# 输出：
# Current version: v1.1.0
# Commit hash: a1b2c3d...
# Build time: 2026-05-06 16:00:00
```

### 配置校验

服务启动时会自动校验配置，如果校验失败，服务将退出并输出错误信息：

- 缺少必要的链配置
- 不支持的链类型
- 缺少必要的 SDK 字段（如 Ethereum 的 `HttpUrl`、ChainMaker 的 `ConfFilePath`、Solana 的 `RpcUrl`）
- 私钥格式无效
- 订阅场景下缺少合约必填字段（如 Ethereum 需要 `ContractAddr`、`GetHistoryEventInterval`、`GetHistoryEventHeightWindow`）

---

## gRPC 客户端集成

### Go 客户端示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    pb "github.com/jackz-jones/blockchain-interactive-service/pb"
)

func main() {
    // 连接 gRPC 服务端
    conn, err := grpc.Dial("localhost:8085", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("连接失败: %v", err)
    }
    defer conn.Close()

    client := pb.NewChainInteractiveClient(conn)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // 示例：调用合约（Invoke 写链）
    resp, err := client.CallContract(ctx, &pb.CallContractRequest{
        RequestId:      "req-001",
        ChainName:      "ethereum01",
        ContractName:   "notification",
        ContractMethod: "sendMessage",
        KvPairs: []*pb.KeyValuePair{
            {Key: "message", Value: []byte("Hello, Blockchain!")},
        },
        MethodType:     pb.MethodType_Invoke,
        WithSyncResult: true,
        TxTimeout:      30,
    })
    if err != nil {
        log.Fatalf("CallContract 失败: %v", err)
    }
    fmt.Printf("返回码: %d, 交易ID: %s, 是否打包中: %v\n", resp.Code, resp.Data.TxId, resp.Data.Pending)

    // 示例：查询交易
    txResp, err := client.GetTxByTxId(ctx, &pb.GetTxByTxIdRequest{
        RequestId: "req-002",
        TxId:      resp.Data.TxId,
        ChainName: "ethereum01",
    })
    if err != nil {
        log.Fatalf("GetTxByTxId 失败: %v", err)
    }
    fmt.Printf("返回码: %d, 是否打包中: %v, 内容: %s\n", txResp.Code, txResp.Data.Pending, txResp.Data.Content)

    // 示例：查询可用链与合约
    availResp, err := client.GetAvailableChainAndContractNames(ctx, &pb.GetAvailableChainAndContractNamesRequest{
        RequestId: "req-003",
    })
    if err != nil {
        log.Fatalf("GetAvailableChainAndContractNames 失败: %v", err)
    }
    for _, chain := range availResp.Data {
        fmt.Printf("链: %s (类型: %s)\n", chain.ChainName, chain.ChainType)
        for _, contract := range chain.ContractDescs {
            fmt.Printf("  合约: %s (地址: %s)\n",
                contract.ContractName, contract.ContractAddress)
        }
    }
}
```

### 使用 TLS 的 gRPC 连接

```go
import (
    "crypto/tls"
    "crypto/x509"
    "io/ioutil"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

func dialWithTLS() *grpc.ClientConn {
    // 加载 CA 证书
    caCert, _ := ioutil.ReadFile("./cert/ca/ca.pem")
    certPool := x509.NewCertPool()
    certPool.AppendCertsFromPEM(caCert)

    // 加载客户端证书和私钥
    clientCert, _ := tls.LoadX509KeyPair("./cert/client/client.pem", "./cert/client/client.key")

    creds := credentials.NewTLS(&tls.Config{
        Certificates: []tls.Certificate{clientCert},
        RootCAs:      certPool,
    })

    conn, _ := grpc.Dial("localhost:8085", grpc.WithTransportCredentials(creds))
    return conn
}
```

### 使用 grpcurl 测试

在 dev/test 模式下，gRPC 反射功能已启用，可以使用 `grpcurl` 进行快速测试：

```bash
# 列出服务
grpcurl -plaintext localhost:8085 list

# 查看服务描述
grpcurl -plaintext localhost:8085 describe proto.ChainInteractive

# 调用 GetAvailableChainAndContractNames
grpcurl -plaintext -d '{"requestId":"test-001"}' localhost:8085 proto.ChainInteractive/GetAvailableChainAndContractNames

# 调用 CallContract（Ethereum Invoke）
grpcurl -plaintext -d '{
  "requestId": "test-002",
  "chainName": "ethereum01",
  "contractName": "notification",
  "contractMethod": "sendMessage",
  "kvPairs": [{"key": "message", "value": "SGVsbG8="}],
  "methodType": 1,
  "withSyncResult": true,
  "txTimeout": 30
}' localhost:8085 proto.ChainInteractive/CallContract

# 调用 GetTxByTxId
grpcurl -plaintext -d '{
  "requestId": "test-003",
  "txId": "0xabc123...",
  "chainName": "ethereum01"
}' localhost:8085 proto.ChainInteractive/GetTxByTxId
```

---

## API 参考

### CallContract

在指定链上调用或查询智能合约。

**请求：`CallContractRequest`**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| requestId | string | 否 | 请求 ID，用于日志追踪 |
| chainName | string | 是 | 链配置名称（如 `ethereum01`） |
| contractName | string | 是 | 合约配置名称（如 `notification`） |
| contractMethod | string | 是 | 要调用的合约方法 |
| kvPairs | KeyValuePair[] | 否 | 方法参数键值对 |
| methodType | MethodType | 是 | `1` = Invoke（写链），`2` = Query（读链） |
| withSyncResult | bool | 否 | 是否同步等待上链确认（默认 false） |
| txTimeout | int64 | 否 | 超时时间（秒，默认 30） |

**响应：`TxResponse`**

| 字段 | 类型 | 说明 |
|---|---|---|
| code | int32 | 返回码（200000 = 成功） |
| msg | string | 错误信息 |
| data | TxData | 交易数据 |

**`TxData` 字段：**

| 字段 | 类型 | 说明 |
|---|---|---|
| chainName | string | 链配置名称 |
| content | string | 交易结果（JSON 字符串） |
| pending | bool | `true` = 尚未确认，`false` = 已上链确认 |
| txId | string | 交易哈希/ID |

**行为细节：**

- **Invoke + `withSyncResult=true`**：服务等待交易上链确认后返回，`pending=false`。
- **Invoke + `withSyncResult=false`**：服务提交交易后立即返回，`pending=true`。
- **Query（methodType=2）**：读取合约状态，不会创建交易，`txId` 可能为空。
- **Ethereum 超时**：同步获取 receipt 超时时，交易已成功提交到节点。响应中会包含 `txId` 和超时错误信息。

### GetTxByTxId

根据交易 ID 查询交易详情和上链状态。

**请求：`GetTxByTxIdRequest`**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| requestId | string | 否 | 请求 ID，用于日志追踪 |
| txId | string | 是 | 交易哈希/ID |
| chainName | string | 是 | 链配置名称 |

**响应：`TxResponse`**（与 CallContract 相同）

`pending` 字段表示交易确认状态：
- `true`：交易待确认（尚未被打包进区块）
- `false`：交易已确认

### GetAvailableChainAndContractNames

返回所有已启用的链及其合约配置信息。

**请求：`GetAvailableChainAndContractNamesRequest`**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| requestId | string | 否 | 请求 ID，用于日志追踪 |

**响应：`GetAvailableChainAndContractNamesResponse`**

| 字段 | 类型 | 说明 |
|---|---|---|
| code | int32 | 返回码 |
| msg | string | 消息 |
| data | ChainAndContractName[] | 链与合约列表 |

**`ChainAndContractName` 字段：**

| 字段 | 类型 | 说明 |
|---|---|---|
| chainName | string | 链配置名称 |
| chainType | ChainType | 链类型枚举（0=Ethereum, 1=Chainmaker, 2=Solana） |
| contractDescs | ContractDesc[] | 合约描述列表 |

**`ContractDesc` 字段：**

| 字段 | 类型 | 说明 |
|---|---|---|
| contractName | string | 合约配置名称 |
| contractAddress | string | 合约地址（Ethereum/Solana） |
| abi | string | 合约 ABI JSON（仅 Ethereum） |

---

## 各链使用说明

### Ethereum

**配置：**

```yaml
ChainConfs:
  ethereum01:
    Enable: true
    ChainType: "ethereum"
    SdkConf:
      EthConf:
        ChainId: 1                                    # 链 ID（1=主网, 5=Goerli 等）
        HttpUrl: "https://mainnet.infura.io/v3/KEY"   # HTTP RPC 端点
        WebsocketUrl: "wss://mainnet.infura.io/ws/v3/KEY"  # WebSocket 端点（用于事件订阅）
        PrivateKey: "hex-私钥"                         # 签名私钥（hex 格式）
```

**CallContract 行为：**

- **Invoke**：通过 HTTP RPC 发送签名交易。如果 `withSyncResult=true`，会轮询等待交易回执。
- **Query**：通过 `eth_call` 调用合约的只读方法。
- **参数传递**：`kvPairs` 根据合约方法名通过 ABI 文件映射到方法的输入参数。

**合约订阅配置：**

```yaml
ContractConfs:
  notification:
    EnableSubscribe: true
    ContractAddr: "0x..."                    # 合约地址
    Abi: ./etc/notification.json             # ABI JSON 文件路径
    DeployBlockHeight: 0                     # 合约初始订阅开始高度
    GetHistoryEventInterval: 500             # 轮询历史事件间隔（毫秒）
    GetHistoryEventHeightWindow: 100         # 每次轮询的区块高度窗口
```

**事件订阅机制：**

Ethereum 客户端使用轮询策略订阅事件：
1. 启动时，从 `DeployBlockHeight`（合约初始订阅开始高度）到当前区块高度批量获取历史事件。
2. 追上最新区块后，切换到基于 WebSocket 的实时事件订阅。
3. 如果 WebSocket 连接断开，会自动重连并从上次处理的区块继续。
4. 事件发布到 Redis，供下游消费者使用。

### ChainMaker

**配置：**

```yaml
ChainConfs:
  chainmaker01:
    Enable: true
    ChainType: "chainmaker"
    SdkConf:
      ConfFilePath: ./etc/chainmaker_sdk_config.yml   # ChainMaker SDK 配置文件
```

`chainmaker_sdk_config.yml` 文件包含链节点地址、用户证书等 ChainMaker SDK 所需的链特定设置。

**CallContract 行为：**

- **Invoke**：向链节点发送交易，可选择等待确认。
- **Query**：读取合约状态，不会创建交易。
- **参数传递**：`kvPairs` 以键值对形式传递给 ChainMaker SDK。

**合约订阅配置：**

```yaml
ContractConfs:
  notification:
    EnableSubscribe: true
    ContractName: "notificationv100"   # 链上合约名称
    DeployBlockHeight: 5               # 合约初始订阅开始高度
```

**事件订阅机制：**

ChainMaker 使用 SDK 内置的事件订阅 API。事件由链节点推送到服务，然后发布到 Redis。

### Solana

**配置：**

```yaml
ChainConfs:
  solana01:
    Enable: true
    ChainType: "solana"
    SdkConf:
      SolanaConf:
        RpcUrl: "https://api.mainnet-beta.solana.com"  # RPC 端点
        PrivateKey: "base58-私钥"                        # 签名私钥（base58 格式）
        CommitmentLevel: "confirmed"                      # processed / confirmed / finalized
        SkipPreflight: false                              # 是否跳过预检
        MaxRetries: 3                                     # 交易重试次数
```

**CallContract 行为：**

- **Invoke**：根据指令规范构建 Solana 交易，签名后发送到 RPC 节点。
  - 指令数据通过 Borsh 序列化根据 `SolanaMethods` 配置构建。
  - 账户元数据从 `Accounts` 配置中解析（支持 `$fromAddress` 占位符）。
  - 如果 `withSyncResult=true`，服务等待交易确认。
- **Query**：使用 `getMultipleAccounts` RPC 读取账户数据，根据方法的 `Discriminator` 进行解码。

**Solana 方法规范（`SolanaMethods`）：**

这是定义 Solana 合约调用编解码方式的关键配置：

```yaml
SolanaMethods:
  # 方法名（与 CallContract 请求中的 contractMethod 对应）
  notify:
    # 8 字节 Anchor discriminator（hex 字符串，16 个 hex 字符）
    Discriminator: "e445a52e51cb9a1d"
    # Borsh 序列化的参数 Schema
    ArgSchema:
      - Name: "msg"           # 必须与请求中 KeyValuePair.Key 对应
        Type: "string"        # 类型：u8, u16, u32, u64, i64, bool, string, pubkey, bytes
      - Name: "amount"
        Type: "u64"
    # Invoke 调用所需的账户列表
    Accounts:
      - Pubkey: "$fromAddress"  # 占位符，表示签名方地址
        IsSigner: true
        IsWritable: true
      - Pubkey: "SomeAccount1111111111111111111111111"
        IsSigner: false
        IsWritable: false

  # 使用 getMultipleAccounts 的查询方法
  getState:
    Discriminator: "0000000000000000"
    # 需要读取的账户地址列表（支持 "$fromAddress" 占位符）
    QueryAccounts:
      - "$fromAddress"
      - "DataAccount1111111111111111111111111111"
```

**支持的 ArgSchema 类型：**

| 类型 | 大小 | 说明 |
|---|---|---|
| u8 | 1 字节 | 无符号 8 位整数 |
| u16 | 2 字节 | 无符号 16 位整数 |
| u32 | 4 字节 | 无符号 32 位整数 |
| u64 | 8 字节 | 无符号 64 位整数 |
| i64 | 8 字节 | 有符号 64 位整数 |
| bool | 1 字节 | 布尔值（0 或 1） |
| string | 4+N 字节 | Borsh 字符串（长度前缀 + UTF-8 数据） |
| pubkey | 32 字节 | Solana 公钥（base58 输入） |
| bytes | 可变 | 原始字节（base64 输入） |

**合约订阅配置：**

```yaml
ContractConfs:
  notification:
    EnableSubscribe: true
    ContractAddr: "program-id-base58"   # Solana 程序 ID（base58 格式）
    DeployBlockHeight: 0                 # 初始订阅开始 Slot 编号
```

**事件订阅机制：**

Solana 使用基于 WebSocket 的日志订阅（`logsSubscribe`）来监控程序事件。事件解析后发布到 Redis。

---

## 事件订阅

### 概述

事件订阅系统允许您在链上合约事件发生时接收实时通知。系统提供两种方式管理订阅：

1. **合约配置页面**：创建合约时直接开启 `EnableSubscribe`
2. **订阅管理页面**：为未开启订阅的合约补充创建订阅

消费端通过 **gRPC 服务端流（Server-Side Streaming）** 实时接收事件推送。

### 架构

```mermaid
sequenceDiagram
    participant C as 消费端 (gRPC Client)
    participant S as Chain Interactive Service
    participant CH as 区块链节点
    participant R as Redis Stream
    
    S->>CH: 订阅合约事件
    CH-->>S: 事件数据
    S->>R: 发布事件到 Redis Stream
    C->>S: SubscribeContractEvents (gRPC Stream)
    S->>R: 读取 Redis Stream 事件
    S-->>C: 流式推送事件
```

### 工作原理

1. 服务启动时调用 `StartSubscribe` 函数，启动调度协程。
2. 调度器每 3 秒检查一次需要订阅的链/合约。
3. 对于已启用订阅的合约（`EnableSubscribe: true`），启动订阅协程。
4. 如果订阅协程退出（错误或断连），`SubscribeFlag` 会被清除，下次调度周期将重新订阅。
5. 服务关闭时，根 context 被取消，传播到所有订阅协程驱动其退出。

### gRPC 流式事件消费

消费端通过 gRPC 服务端流接口 `SubscribeContractEvents` 实时接收事件：

```protobuf
// 请求
message SubscribeContractEventsRequest {
  string requestId = 1;     // 请求 ID（日志追踪）
  string chainName = 2;     // 链名称（必填）
  string contractName = 3;  // 合约名称（可选）
  string contractAddr = 4;  // 合约地址（可选）
}

// 响应流
message ContractEventResponse {
  string eventName = 1;       // 事件名称
  string contractAddress = 2; // 合约地址
  string txHash = 3;          // 交易哈希
  uint64 blockNumber = 4;     // 区块号
  string data = 5;            // 事件数据 JSON
  int64 timestamp = 6;        // 时间戳（Unix 秒）
}
```

**Go 客户端示例：**

```go
import (
    pb "github.com/jackz-jones/blockchain-interactive-service/pb"
)

// 建立 gRPC 连接（需携带认证信息）
stream, err := client.SubscribeContractEvents(ctx, &pb.SubscribeContractEventsRequest{
    ChainName:    "my-chain",
    ContractName: "my-contract",
})
if err != nil {
    log.Fatal(err)
}

// 持续接收事件
for {
    event, err := stream.Recv()
    if err != nil {
        log.Printf("stream ended: %v", err)
        break
    }
    log.Printf("received event: %s, data: %s", event.EventName, event.Data)
}
```

**注意事项：**
- 每个租户最大并发流连接数为 10（可配置）
- 客户端断开后需自行实现重连逻辑
- 合约必须先开启事件订阅（`EnableSubscribe: true`）才能消费事件

### HTTP 订阅管理接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/events/subscriptions` | 获取已开启订阅的合约列表 |
| GET | `/api/v1/events/available-contracts` | 获取可订阅的合约列表（未开启订阅的） |
| POST | `/api/v1/events/subscribe-by-contract` | 为合约开启事件订阅 |
| DELETE | `/api/v1/events/subscribe-by-contract/:contractConfigId` | 取消合约事件订阅 |

### Redis 部署模式

服务支持三种 Redis 部署模式：

| 模式 | ConfType | 说明 |
|---|---|---|
| 单节点 | `node` | 单个 Redis 实例 |
| 集群 | `cluster` | Redis Cluster 多节点集群 |
| 哨兵 | `sentinel` | Redis Sentinel 高可用模式 |

```yaml
SubscribeConf:
  ConfType: sentinel
  RedisAddr: "sentinel1:26379,sentinel2:26379,sentinel3:26379"
  RedisUserName: ""
  RedisPassword: "your-password"
  MasterName: "mymaster"    # 哨兵模式必填
```

---

## TLS 配置

### 启用 gRPC TLS

1. 生成证书：

```bash
make gen-cert
# 证书将生成在 ./cert/ 目录下
```

2. 配置服务：

```yaml
GrpcConf:
  CaCertFile: ./cert/ca/ca.pem
  ServerCertFile: ./cert/chain-service/server.pem
  ServerKeyFile: ./cert/chain-service/server.key
```

3. 客户端需要加载 CA 证书和客户端证书/私钥以实现双向 TLS 认证。

### 禁用 TLS

将证书路径留空即可禁用 TLS：

```yaml
GrpcConf:
  CaCertFile: ""
  ServerCertFile: ""
  ServerKeyFile: ""
```

---

## 监控

### 健康检查

服务提供健康检查端点：

```bash
curl http://localhost:6061/healthz
# 响应：OK
```

### Prometheus 指标

Prometheus 指标可在以下端点获取：

```bash
curl http://localhost:6061/metrics
```

### OpenTelemetry 链路追踪

在服务配置中开启追踪：

```yaml
Telemetry:
  Disabled: false
  Name: chain.rpc
  Endpoint: http://jaeger:14268/api/traces
  Sampler: 1.0
  Batcher: jaeger  # jaeger / zipkin / otlpgrpc / otlphttp
```

### DevServer 配置

```yaml
DevServer:
  Enable: true
  Port: 6061
  HealthPath: "/healthz"
  HealthResponse: "OK"
  MetricsPath: "/metrics"
```

---

### 用量统计

用量统计页面展示租户的调用趋势和配额使用情况。

#### 趋势数据字段

| 字段 | 类型 | 说明 |
|------|------|------|
| dates | string[] | 日期序列（MM/DD 格式） |
| calls | int64[] | 每日总调用量 |
| success | int64[] | 每日成功次数 |
| failed | int64[] | 每日失败次数 |
| invoke | int64[] | 每日 Invoke（写链）调用量 |
| query | int64[] | 每日 Query（读链）调用量 |
| quota_limit | int64 | 月配额上限 |
| quota_used | int64 | 已使用月配额 |

#### Invoke/Query 分类统计

用量统计趋势数据中新增了 `invoke` 和 `query` 两个维度，用于区分写链操作和读链操作：

- **Invoke（写链）**：合约写操作，如发送交易、修改链上状态，会计入配额和计费
- **Query（读链）**：合约读操作，如查询链上状态、读取合约数据，不消耗配额

计算关系：`query = calls - invoke`（当结果为负数时置为 0）

#### API 接口

**`GET /api/v1/dashboard/usage-stats?period=30d`**

请求参数：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| period | string | 30d | 统计周期：7d / 30d / 90d |

---

## 错误处理

### 返回码规则

- `200000`：成功
- `600xxx`：服务级错误

| 返回码 | 常量 | 说明 |
|---|---|---|
| 200000 | Success | 操作成功 |
| 600000 | ErrUnknownChainType | 配置中的 `ChainType` 不被识别 |
| 600002 | ErrGetSDKClient | 创建或获取 SDK 客户端失败 |
| 600003 | ErrGetTxByTxId | 根据交易 ID 查询交易失败 |
| 600004 | ErrSendTransaction | 发送交易到链上失败 |
| 600005 | ErrReadAbiJsonFile | 读取 ABI JSON 文件失败（Ethereum） |
| 600006 | ErrChainNotExist | 指定的 `chainName` 在配置中不存在 |
| 600007 | ErrChainNotEnable | 指定的链存在但未启用 |

### Ethereum 交易超时

在 Ethereum 上使用 `withSyncResult=true` 调用 `CallContract` 时，如果在超时时间内未获取到交易回执，服务会返回错误码 `600004`，并附带消息：

```
sync to get tx receipt timeout, maybe try it later
```

在这种情况下，交易**已成功提交**到节点。响应数据中包含 `txId`，您可以稍后使用 `GetTxByTxId` 查询交易状态。

---

## 合约调用同步/异步模式

### 概述

合约写链调用（Invoke）支持两种模式：

| 模式 | 参数 | 行为 | 适用场景 |
|------|------|------|---------|
| **异步模式**（默认） | `sync=false` | 提交交易后立即返回 `tx_id`，不等待区块确认 | 高吞吐、对延迟敏感的场景 |
| **同步模式** | `sync=true` | 等待交易被区块打包确认后再返回完整 receipt | 需要即时确认的场景 |

### 异步模式流程

```mermaid
sequenceDiagram
    participant 前端/客户端
    participant 后端服务
    participant 区块链节点

    前端/客户端->>后端服务: POST /api/v1/contract/call (sync=false)
    后端服务->>区块链节点: 签名并提交交易
    区块链节点-->>后端服务: 返回 txId (交易已进入 mempool)
    后端服务-->>前端/客户端: {tx_id: "0x...", pending: true} (1~3秒内响应)

    Note over 前端/客户端: 开始轮询交易状态

    loop 每3秒轮询一次 (最多20次 ≈ 60秒)
        前端/客户端->>后端服务: GET /api/v1/tx/{txId}?chain_name=xxx
        后端服务->>区块链节点: 查询交易状态 (TransactionByHash / GetTxByTxId)
        区块链节点-->>后端服务: pending=true / confirmed
        后端服务-->>前端/客户端: {confirmed: true/false, result: "..."}
    end

    Note over 前端/客户端: confirmed=true → 交易已上链确认
```

### 同步模式流程

```mermaid
sequenceDiagram
    participant 前端/客户端
    participant 后端服务
    participant 区块链节点

    前端/客户端->>后端服务: POST /api/v1/contract/call (sync=true, tx_timeout=30)
    后端服务->>区块链节点: 签名并提交交易
    区块链节点-->>后端服务: 返回 txId

    loop 等待区块确认 (直到超时)
        后端服务->>区块链节点: 轮询 TransactionReceipt
        区块链节点-->>后端服务: receipt (成功/失败)
    end

    后端服务-->>前端/客户端: {tx_id: "0x...", pending: false, result: "{receipt}"}
```

### HTTP API 请求参数

**`POST /api/v1/contract/call`**

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| chain_name | string | 是 | - | 链配置名称 |
| contract_name | string | 是 | - | 合约配置名称 |
| method | string | 是 | - | 合约方法名 |
| method_type | int | 否 | 1 | 1=写链(Invoke)，2=读链(Query) |
| params | map | 否 | {} | 方法参数键值对 |
| **sync** | bool | 否 | **false** | 是否同步等待上链确认 |
| **tx_timeout** | int | 否 | 10 | 同步模式下的超时时间（秒，范围 5~60） |

### 响应格式

**异步模式响应（sync=false）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "tx_id": "0xabc123def456...",
    "result": "",
    "pending": true,
    "duration": "1.2s"
  }
}
```

**同步模式响应（sync=true）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "tx_id": "0xabc123def456...",
    "result": "{\"status\":1,\"blockNumber\":12345678,...}",
    "pending": false,
    "duration": "8.5s"
  }
}
```

### 交易状态查询接口

**`GET /api/v1/tx/:txId?chain_name=xxx`**

用于异步模式下轮询交易确认状态。

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "result": "{\"txId\":\"0x...\",\"status\":1,\"from\":\"0x...\",\"to\":\"0x...\"}",
    "confirmed": true
  }
}
```

| 字段 | 说明 |
|------|------|
| `confirmed: false` | 交易仍在 pending 状态，尚未被区块打包 |
| `confirmed: true` | 交易已被区块确认，`result` 中包含完整交易详情 |

### Web 控制台前端轮询机制

Web 管理控制台的"合约调用"页面内置了自动轮询逻辑：

| 状态 | 前端展示 | 说明 |
|------|---------|------|
| **PENDING** | 🔄 旋转图标 + "轮询确认中..." | 交易已提交，前端自动每 3 秒查询一次 |
| **CONFIRMED** | ✅ "已确认上链" | 轮询到 `confirmed=true`，展示交易详情 |
| **TIMEOUT** | ⚠️ "轮询超时，请手动查询交易状态" | 60 秒内未确认，停止轮询 |

**轮询参数：**

- 轮询间隔：3 秒
- 最大轮询次数：20 次（约 60 秒）
- 组件卸载时自动清除定时器，避免内存泄漏
- 查询失败不中断轮询，继续重试

### 各链确认时间参考

| 链 | 典型确认时间 | 建议模式 |
|---|---|---|
| **Ethereum 主网** | 12~15 秒 | 异步 + 轮询 |
| **Ethereum L2（Polygon/Arbitrum）** | 2~5 秒 | 同步（tx_timeout=10） |
| **ChainMaker** | 1~3 秒 | 同步（tx_timeout=10） |
| **Solana** | 0.4~2 秒 | 同步（tx_timeout=10） |

### 客户端集成建议

**Go 客户端异步轮询示例：**

```go
// 1. 异步提交交易
resp, err := client.CallContract(ctx, &pb.CallContractRequest{
    ChainName:      "ethereum01",
    ContractName:   "notification",
    ContractMethod: "sendMessage",
    KvPairs:        []*pb.KeyValuePair{{Key: "msg", Value: []byte("Hello")}},
    MethodType:     pb.MethodType_Invoke,
    WithSyncResult: false,  // 异步模式
})
txId := resp.Data.TxId

// 2. 轮询交易状态
ticker := time.NewTicker(3 * time.Second)
defer ticker.Stop()
timeout := time.After(60 * time.Second)

for {
    select {
    case <-ticker.C:
        txResp, err := client.GetTxByTxId(ctx, &pb.GetTxByTxIdRequest{
            TxId:      txId,
            ChainName: "ethereum01",
        })
        if err == nil && !txResp.Data.Pending {
            fmt.Printf("交易已确认: %s\n", txResp.Data.Content)
            return
        }
    case <-timeout:
        fmt.Println("轮询超时，请稍后手动查询")
        return
    }
}
```

**curl 异步调用示例：**

```bash
# 步骤 1：异步提交交易
TX_ID=$(curl -s -X POST http://localhost:8080/api/v1/contract/call \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "chain_name": "ethereum01",
    "contract_name": "notification",
    "method": "sendMessage",
    "params": {"message": "Hello"},
    "sync": false
  }' | jq -r '.data.tx_id')

echo "交易已提交: $TX_ID"

# 步骤 2：轮询交易状态
for i in $(seq 1 20); do
  sleep 3
  RESULT=$(curl -s -H "X-API-Key: your-api-key" \
    "http://localhost:8080/api/v1/tx/${TX_ID}?chain_name=ethereum01")
  CONFIRMED=$(echo $RESULT | jq -r '.data.confirmed')
  if [ "$CONFIRMED" = "true" ]; then
    echo "交易已确认！"
    echo $RESULT | jq '.data.result'
    break
  fi
  echo "第 ${i} 次轮询：交易仍在 pending..."
done
```

---

## 最佳实践

### 配置

1. **先禁用未使用的链**：将不活跃使用的链设为 `Enable: false`，避免不必要的 SDK 客户端初始化。
2. **设置合理的超时时间**：根据链的典型确认时间设置 `txTimeout`。
3. **使用环境特定配置**：为 dev/test/pre/prod 环境维护独立的配置文件。

### 性能

1. **高吞吐场景使用异步模式**：不需要即时确认时设置 `withSyncResult=false`，然后通过 `GetTxByTxId` 单独轮询。
2. **调整 Ethereum 事件轮询参数**：根据链的出块速率调整 `GetHistoryEventInterval` 和 `GetHistoryEventHeightWindow`。
3. **生产环境使用 Redis 集群**：为高可用事件订阅使用 Redis 集群或哨兵模式。

### 可靠性

1. **启用订阅自动恢复**：服务在订阅协程退出时会自动重新订阅，无需手动干预。
2. **监控健康检查**：对 `/healthz` 端点设置监控告警，及时发现服务问题。
3. **生产环境启用 TLS**：在生产环境中始终为 gRPC 通信启用 TLS 加密。

### 开发

1. **开发模式使用 gRPC 反射**：服务在 dev/test 模式下启用 gRPC 反射，支持 `grpcurl` 等工具发现服务。
2. **提交前运行测试**：使用 `make pre-commit` 运行代码检查、测试和注释覆盖率检查。
3. **版本化构建**：构建系统会注入版本号、commit hash 和构建时间到二进制文件中。

---

## 多租户 HTTP API

服务提供 RESTful HTTP API Gateway（默认端口：8080），用于多租户管理和合约操作。

所有 HTTP Handler 通过 `goctl` 从 API 定义文件 `api/chaininteractive.api` 生成。

### 认证方式

所有 HTTP API 请求需要在请求头中携带 `X-API-Key`：

```bash
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/chains
```

### 完整 API 路由表

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/contract/call` | 调用/查询合约 |
| GET | `/api/v1/tx/:txId` | 根据 ID 查询交易 |
| GET | `/api/v1/chains` | 获取可用链列表 |
| GET | `/api/v1/chains/:chainName/status` | 获取链连接状态 |
| POST | `/api/v1/events/subscribe` | 订阅合约事件 |
| GET | `/api/v1/events/poll` | 轮询已订阅事件 |
| DELETE | `/api/v1/events/subscribe/:subscriptionId` | 取消订阅 |
| POST | `/api/v1/tenants` | 创建租户 |
| GET | `/api/v1/tenants/:id` | 获取租户详情 |
| GET | `/api/v1/tenants` | 租户列表 |
| POST | `/api/v1/tenants/:id/disable` | 禁用租户 |
| POST | `/api/v1/tenants/:id/enable` | 启用租户 |
| POST | `/api/v1/api-keys` | 创建 API Key |
| GET | `/api/v1/api-keys` | API Key 列表 |
| POST | `/api/v1/chain-configs` | 创建链配置 |
| GET | `/api/v1/chain-configs` | 链配置列表 |
| GET | `/api/v1/chain-configs/:id` | 获取链配置详情 |
| PUT | `/api/v1/chain-configs/:id` | 更新链配置 |
| DELETE | `/api/v1/chain-configs/:id` | 删除链配置 |
| POST | `/api/v1/chain-configs/:id/test-connection` | 测试链连接 |
| POST | `/api/v1/chain-configs/:chainConfigId/contracts` | 创建合约配置 |
| GET | `/api/v1/chain-configs/:chainConfigId/contracts` | 合约配置列表 |
| GET | `/api/v1/chain-configs/:chainConfigId/contracts/:id` | 获取合约配置 |
| PUT | `/api/v1/chain-configs/:chainConfigId/contracts/:id` | 更新合约配置 |
| DELETE | `/api/v1/chain-configs/:chainConfigId/contracts/:id` | 删除合约配置 |
| GET | `/api/v1/users` | 用户列表 |
| GET | `/api/v1/dashboard/overview` | 仪表盘概览 |
| GET | `/api/v1/dashboard/call-logs` | 调用日志（可筛选） |
| GET | `/api/v1/dashboard/usage-stats` | 用量统计 |
| GET | `/api/v1/dashboard/bills` | 账单记录 |
| GET | `/api/v1/dashboard/audit-logs` | 审计日志 |

### 租户管理

```bash
# 创建租户
curl -X POST http://localhost:8080/api/v1/tenants \
  -H "Content-Type: application/json" \
  -H "X-API-Key: admin-api-key" \
  -d '{"name": "my-company", "email": "admin@company.com", "phone": "13800138000", "password": "secure-pass", "plan": "developer"}'

# 租户列表
curl -H "X-API-Key: admin-api-key" "http://localhost:8080/api/v1/tenants?page=1&page_size=10"

# 获取租户详情
curl -H "X-API-Key: admin-api-key" http://localhost:8080/api/v1/tenants/1

# 禁用租户
curl -X POST http://localhost:8080/api/v1/tenants/1/disable \
  -H "X-API-Key: admin-api-key"

# 启用租户
curl -X POST http://localhost:8080/api/v1/tenants/1/enable \
  -H "X-API-Key: admin-api-key"
```

### API Key 管理

```bash
# 创建 API Key
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Content-Type: application/json" \
  -H "X-API-Key: admin-api-key" \
  -d '{"name": "production-key", "permissions": "contract:call,tx:query", "ip_whitelist": "10.0.0.0/8", "expires_in": 86400}'

# API Key 列表
curl -H "X-API-Key: admin-api-key" "http://localhost:8080/api/v1/api-keys?page=1&page_size=10"
```

### 链配置管理

```bash
# 创建 Ethereum 链配置
curl -X POST http://localhost:8080/api/v1/chain-configs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: admin-api-key" \
  -d '{
    "chain_name": "eth-mainnet",
    "chain_type": "ethereum",
    "enable": true,
    "eth_chain_id": 1,
    "http_url": "https://mainnet.infura.io/v3/YOUR_KEY",
    "websocket_url": "wss://mainnet.infura.io/ws/v3/YOUR_KEY",
    "private_key": "hex-私钥"
  }'

# 创建 ChainMaker 链配置
curl -X POST http://localhost:8080/api/v1/chain-configs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: admin-api-key" \
  -d '{
    "chain_name": "chainmaker01",
    "chain_type": "chainmaker",
    "enable": true,
    "chain_id": "chain1",
    "auth_type": "permissionedWithCert",
    "org_id": "wx-org1.chainmaker.org",
    "sign_key": "-----BEGIN EC PRIVATE KEY-----...",
    "sign_cert": "-----BEGIN CERTIFICATE-----...",
    "nodes": [{"node_addr": "127.0.0.1:12301", "conn_cnt": 10}]
  }'

# 创建 Solana 链配置
curl -X POST http://localhost:8080/api/v1/chain-configs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: admin-api-key" \
  -d '{
    "chain_name": "solana-devnet",
    "chain_type": "solana",
    "enable": true,
    "sol_rpc_url": "https://api.devnet.solana.com",
    "sol_private_key": "base58-私钥",
    "commitment_level": "confirmed",
    "max_retries": 3
  }'

# 链配置列表
curl -H "X-API-Key: admin-api-key" http://localhost:8080/api/v1/chain-configs

# 获取链配置详情
curl -H "X-API-Key: admin-api-key" http://localhost:8080/api/v1/chain-configs/1

# 更新链配置
curl -X PUT http://localhost:8080/api/v1/chain-configs/1 \
  -H "Content-Type: application/json" \
  -H "X-API-Key: admin-api-key" \
  -d '{"enable": false, "chain_name": "eth-mainnet", "chain_type": "ethereum"}'

# 测试链连接
curl -X POST http://localhost:8080/api/v1/chain-configs/1/test-connection \
  -H "X-API-Key: admin-api-key"

# 删除链配置
curl -X DELETE http://localhost:8080/api/v1/chain-configs/1 \
  -H "X-API-Key: admin-api-key"
```

### 合约配置管理

```bash
# 创建合约配置（在链配置 ID 1 下）
curl -X POST http://localhost:8080/api/v1/chain-configs/1/contracts \
  -H "Content-Type: application/json" \
  -H "X-API-Key: admin-api-key" \
  -d '{
    "contract_name": "notification",
    "contract_addr": "0x1234...",
    "abi_json": "[{...}]",
    "enable_subscribe": true,
    "extra_conf": "{\"deploy_block_height\": 0}"
  }'

# 合约配置列表
curl -H "X-API-Key: admin-api-key" http://localhost:8080/api/v1/chain-configs/1/contracts

# 获取合约配置详情
curl -H "X-API-Key: admin-api-key" http://localhost:8080/api/v1/chain-configs/1/contracts/1

# 更新合约配置
curl -X PUT http://localhost:8080/api/v1/chain-configs/1/contracts/1 \
  -H "Content-Type: application/json" \
  -H "X-API-Key: admin-api-key" \
  -d '{"contract_name": "notification", "enable_subscribe": false}'

# 删除合约配置
curl -X DELETE http://localhost:8080/api/v1/chain-configs/1/contracts/1 \
  -H "X-API-Key: admin-api-key"
```

### 通过 HTTP 调用合约

```bash
# 调用合约（Invoke）
curl -X POST http://localhost:8080/api/v1/contract/call \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "chain_name": "ethereum01",
    "contract_name": "notification",
    "method": "sendMessage",
    "params": {"message": "Hello, Blockchain!"}
  }'

# 查询交易
curl -H "X-API-Key: your-api-key" \
  "http://localhost:8080/api/v1/tx/0xabc123...?chain_name=ethereum01"

# 获取可用链列表
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/chains

# 获取链状态
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/chains/ethereum01/status
```

### 通过 HTTP 订阅事件

```bash
# 订阅合约事件
curl -X POST http://localhost:8080/api/v1/events/subscribe \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "chain_name": "ethereum01",
    "contract_name": "notification",
    "contract_addr": "0x1234..."
  }'

# 轮询事件（返回缓冲的事件）
curl -H "X-API-Key: your-api-key" \
  "http://localhost:8080/api/v1/events/poll?subscription_id=sub-123&count=10"

# 取消订阅
curl -X DELETE http://localhost:8080/api/v1/events/subscribe/sub-123 \
  -H "X-API-Key: your-api-key"
```

### 用户管理

```bash
# 用户列表
curl -H "X-API-Key: admin-api-key" "http://localhost:8080/api/v1/users?page=1&page_size=10"
```

### 管理后台 API

```bash
# 获取仪表盘概览
curl -H "X-API-Key: admin-api-key" http://localhost:8080/api/v1/dashboard/overview

# 查询调用日志（支持筛选）
curl -H "X-API-Key: admin-api-key" \
  "http://localhost:8080/api/v1/dashboard/call-logs?page=1&page_size=20&chain_name=ethereum01&status=success&start_time=2026-01-01&end_time=2026-12-31"

# 获取用量统计
curl -H "X-API-Key: admin-api-key" http://localhost:8080/api/v1/dashboard/usage-stats

# 查询账单
curl -H "X-API-Key: admin-api-key" \
  "http://localhost:8080/api/v1/dashboard/bills?page=1&page_size=10"

# 查询审计日志
curl -H "X-API-Key: admin-api-key" \
  "http://localhost:8080/api/v1/dashboard/audit-logs?page=1&page_size=20&action=CallContract&start_time=2026-01-01"
```

---

## Web 管理控制台

服务内置了基于 React 19 + Vite + Ant Design 5.x 构建的现代化 Web 管理控制台。

### 启动控制台

```bash
cd web
npm install
npm run dev    # 开发模式：http://localhost:5173
npm run build  # 生产构建
```

### 功能页面

- **仪表盘概览**：关键指标卡片、调用趋势图表（ECharts）
- **链配置管理**：可视化 CRUD Ethereum/ChainMaker/Solana 配置，支持连接测试
- **合约配置管理**：按链管理合约，ABI 编辑器，订阅开关
- **合约调用**：交互式调用，内置 Monaco JSON 编辑器编辑参数
- **交易查询**：搜索和检视交易详情
- **事件订阅**：订阅/轮询/取消订阅，实时事件展示
- **租户管理**：创建、启用/禁用租户
- **API Key 管理**：生成带权限和 IP 白名单的密钥
- **用户管理**：查看用户列表
- **调用日志**：可筛选的调用历史，状态指示器
- **用量统计**：时间序列用量分析图表
- **账单**：计费记录表格
- **审计日志**：安全审计追踪，支持操作类型筛选
- **系统设置**：系统配置页面

### 技术栈

| 技术 | 用途 |
|------|------|
| React 19 | UI 框架 |
| Vite 8 | 构建工具 |
| Ant Design 5.x | 组件库 |
| ECharts | 数据可视化 |
| Monaco Editor | JSON/代码编辑 |
| Zustand | 状态管理 |
| React Router 7 | 客户端路由 |
| Axios | HTTP 客户端 |

---

## 相关文档

- **[架构文档（中文）](architecture_cn.md)** — 系统架构、模块设计、部署架构图
- **[Architecture Document (EN)](architecture_en.md)** — System architecture, module design, deployment diagrams
