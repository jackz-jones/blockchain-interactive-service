**English** | **[中文](README_CN.md)**

# Chain Interactive Service

A universal blockchain interaction service that provides a unified gRPC interface to interact with multiple blockchains, abstracting away the underlying chain differences so that upper-layer services don't need to care about chain-specific implementation details.

## Features

- 🔗 **Multi-Chain Support**: Unified interface for ChainMaker, Ethereum, and Solana, with ongoing expansion
- 📝 **Contract Invocation**: Supports both Invoke (write) and Query (read) call modes
- 🔍 **Transaction Query**: Query transaction details and on-chain status by transaction ID
- 📡 **Event Subscription**: Subscribe to contract events and receive real-time notifications for on-chain contract changes
- ⚡ **Sync/Async**: Contract calls support both synchronous waiting and asynchronous return
- 🔒 **gRPC Security**: Supports TLS mutual authentication for gRPC communication
- 📊 **Monitoring & Tracing**: Built-in health checks, Prometheus metrics, and OpenTelemetry distributed tracing

## Supported Chains

| Chain | Type | Contract Call | Transaction Query | Event Subscription |
|---|---|---|---|---|
| **ChainMaker** | Consortium | ✅ | ✅ | ✅ |
| **Ethereum** | Public | ✅ | ✅ | ✅ |
| **Solana** | Public | ✅ | ✅ | ✅ |

> 🚧 More mainstream blockchains will be added gradually.

## Project Structure

```
.
├── chaininteractive.go         # Service entry point
├── chaininteractive/           # Business logic layer
├── internal/
│   ├── config/                 # Configuration definitions
│   ├── logic/                  # Request handling logic
│   ├── sdk/                    # Chain SDK wrappers
│   │   ├── interface.go        # Unified chain interface definition
│   │   ├── chainmakerclient.go # ChainMaker implementation
│   │   ├── ethereumclient.go   # Ethereum implementation
│   │   └── solanaclient.go     # Solana implementation
│   ├── server/                 # gRPC server
│   ├── svc/                    # Service context
│   └── util/                   # Utility functions
├── proto/                      # Protobuf definitions
├── pb/                         # Generated Protobuf Go code
├── etc/                        # Configuration files
├── docker/                     # Docker build files
├── scripts/                    # Scripts and tools
└── Makefile                    # Build commands
```

## Quick Start

### Prerequisites

- Go 1.22+
- Redis (required for event subscription)

### Build & Run

```bash
# Build
make build

# Run directly
make start-service

# Or use the compiled binary
./chain-interactive-service -f etc/chaininteractive.yaml
```

### Configuration

The configuration file is located at `etc/chaininteractive.yaml`. Key configuration items are as follows:

#### Service Base Configuration

```yaml
Name: chaininteractive.rpc
ListenOn: 0.0.0.0:8085
Timeout: 30000
Mode: dev   # dev / test / pre / prod
```

#### Chain Configuration

Each chain is defined as an independent configuration block under `ChainConfs`, distinguished by `ChainType`:

**Ethereum Example:**

```yaml
ChainConfs:
  ethereum01:
    Enable: true
    ChainType: "ethereum"
    SdkConf:
      EthConf:
        ChainId: 1
        HttpUrl: "https://mainnet.infura.io/v3/YOUR_KEY"
        WebsocketUrl: "wss://mainnet.infura.io/ws/v3/YOUR_KEY"
        PrivateKey: "your-private-key-hex"
        GasLimit: 1000000
    ContractConfs:
      notification:
        EnableSubscribe: true
        ContractType: "notification"
        ContractAddr: "0x..."
        Abi: ./etc/notification.json
        DeployBlockHeight: 0
```

**ChainMaker Example:**

```yaml
ChainConfs:
  chainmaker01:
    Enable: true
    ChainType: "chainmaker"
    SdkConf:
      ConfFilePath: ./etc/chainmaker_sdk_config.yml
    ContractConfs:
      notification:
        EnableSubscribe: true
        ContractType: "notification"
        ContractName: "notificationv100"
        DeployBlockHeight: 5
```

**Solana Example:**

```yaml
ChainConfs:
  solana01:
    Enable: true
    ChainType: "solana"
    SdkConf:
      SolanaConf:
        RpcUrl: "https://api.mainnet-beta.solana.com"
        WsUrl: "wss://api.mainnet-beta.solana.com/"
        PrivateKey: "your-private-key-base58"
        CommitmentLevel: "confirmed"
        SkipPreflight: false
        MaxRetries: 3
    ContractConfs:
      notification:
        EnableSubscribe: true
        ContractType: "notification"
        ContractAddr: "program-id-base58"
        DeployBlockHeight: 0
```

#### Event Subscription Configuration

Event subscription depends on Redis, configured under `SubscribeConf`:

```yaml
SubscribeConf:
  ConfType: node          # node / cluster / sentinel
  RedisAddr: "127.0.0.1:6379"
  RedisUserName: ""
  RedisPassword: ""
  MasterName: ""          # Used in sentinel mode
```

## gRPC API

The service provides the following gRPC interfaces:

### CallContract - Contract Invocation

```protobuf
rpc CallContract(CallContractRequest) returns (TxResponse);
```

| Parameter | Type | Description |
|---|---|---|
| chainName | string | Chain configuration name |
| contractName | string | Contract configuration name |
| contractMethod | string | Contract method name |
| kvPairs | KeyValuePair[] | Contract parameter key-value pairs |
| methodType | MethodType | 1=Invoke (write), 2=Query (read) |
| withSyncResult | bool | Whether to wait for transaction result synchronously |
| txTimeout | int64 | Transaction timeout in seconds |

### GetTxByTxId - Query Transaction

```protobuf
rpc GetTxByTxId(GetTxByTxIdRequest) returns (TxResponse);
```

| Parameter | Type | Description |
|---|---|---|
| txId | string | Transaction ID |
| chainName | string | Chain configuration name |

### GetAvailableChainAndContractNames - Query Available Chains & Contracts

```protobuf
rpc GetAvailableChainAndContractNames(GetAvailableChainAndContractNamesRequest) returns (GetAvailableChainAndContractNamesResponse);
```

Returns all enabled chains and their associated contract configurations in the current service.

## Development Guide

### Generate Protobuf Code

```bash
make gen-code
```

### Run Tests

```bash
make ut
```

### Code Lint

```bash
make lint
```

### Adding a New Chain

1. Create a new chain client file under `internal/sdk/` that implements the `ChainSdkInterface` interface:

```go
type ChainSdkInterface interface {
    CallContract(methodType pb.MethodType, contractConfigName, method string,
        args []*pb.KeyValuePair, txTimeout int64, withSyncResult bool) (string, string, error)
    GetTxByTxId(txId string) (string, bool, error)
    Stop() error
    SubscribeContractEvent(contractConf config.ContractConf, chainConfName,
        contractConfName, chainType, contractType string) error
}
```

2. Add the new chain's configuration structure in `internal/config/config.go`
3. Register the new chain's initialization logic in `internal/svc/servicecontext.go`
4. Add a configuration template in `etc/chaininteractive.yaml`
5. Add the new chain type to the `ChainType` enum in `proto/chaininteractive.proto`

### Docker Build

```bash
make build-docker
```

## Tech Stack

- **Framework**: go-zero
- **Communication**: gRPC + Protobuf
- **Chain SDKs**:
  - ChainMaker: `chainmaker.org/chainmaker/sdk-go/v2`
  - Ethereum: `github.com/ethereum/go-ethereum`
  - Solana: `github.com/gagliardetto/solana-go`
- **Cache**: Redis (event subscription)
- **Monitoring**: OpenTelemetry + Prometheus
- **Logging**: go-zero built-in log package

## License

[Apache License 2.0](LICENSE)

This project is licensed under the Apache License 2.0. You are free to use, modify, and distribute this software under the following conditions:

- ✅ Commercial use, modification, distribution, and private use
- ✅ Patent license protection granted
- ⚠️ Must preserve copyright and license notices
- ⚠️ Modified files must indicate changes
- ⚠️ Must include a copy of the license when distributing
- ❌ No warranty provided
- ❌ Author assumes no liability