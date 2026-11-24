# 数据库规范

> 状态：生效 | 维护者：jackz | 最后更新：2026-11-23（sop.init 填实）

## 范围

覆盖本服务所有持久化存储：MySQL / PostgreSQL / Kingbase（GORM）与 Redis。

## 1. 选型与版本

| 存储 | 用途 | 版本 | 部署形态 |
|------|------|------|----------|
| MySQL | 主业务数据（可切换） | 8.0 | 测试环境 |
| PostgreSQL | 主业务数据（helm 默认 `postgresql:5432`） | 14+ | K8s 外部服务 |
| Kingbase（人大金仓） | 信创环境主业务数据 | V8 | 按需 |
| Redis | 事件推送 Stream / 订阅断点 KV | 6.x / 7.x | 单机 / Cluster / Sentinel（`SubscribeConf` 支持） |

> `DatabaseConf.Type` 决定驱动；DSN 与连接池在 `etc/chaininteractive.yaml`。

## 2. 表设计规范

### 命名
- 表名：`snake_case`，**复数** 全项目一致（tenants / users / api_keys …）
- 字段名：`snake_case`
- 索引名：
  - 普通索引 `idx_{{表}}_{{字段}}`（现有示例：`idx_tenant_chain`、`idx_calllog_tenant_created`）
  - 唯一索引 `uk_{{字段}}`
  - 外键 `fk_{{from}}_{{to}}`

### 必备字段
每张业务表必须包含：

```sql
id          BIGINT UNSIGNED  PRIMARY KEY AUTO_INCREMENT,
created_at  DATETIME(3)      NOT NULL,
updated_at  DATETIME(3)      NOT NULL,
is_deleted  TINYINT(1)       NOT NULL DEFAULT 0,  -- 软删除（如使用）
```

> 软删除字段名 `is_deleted` 与 `deleted_at` 二选一全项目一致。

### 类型选择
- 主键：`BIGINT UNSIGNED`（业务库）或 `UUID`（分布式）
- 时间：`DATETIME(3)` 或 `TIMESTAMP`，**禁用** `INT` 存时间
- 金额：`DECIMAL(N, 4)`，**禁用** `FLOAT` / `DOUBLE`（精度丢失）
- 枚举：`TINYINT` + 代码常量 或 `VARCHAR(32)`
- JSON：仅在不需要索引/聚合的属性配置时使用；**禁止**用 JSON 存关系数据
- 字符串：明确长度上限（`VARCHAR(N)`），**避免** `TEXT` / `BLOB`（除非确实需要）

### 索引
- 单表索引数量：≤**5**（含主键）
- 复合索引字段数：≤**3**
- 必须为高频查询创建联合索引（最左前缀原则）
- **禁止**在选择性差的字段（性别、状态布尔）上单独建索引
- 外键字段必须有索引
- 必须分析 EXPLAIN，确认走索引

### 字符集与排序
- 字符集：`utf8mb4`
- 排序：`utf8mb4_0900_ai_ci`（MySQL 8）或 `utf8mb4_unicode_ci`

## 3. ORM 与查询

- ORM：**GORM v1.25.10**（连接初始化 `internal/store/db.go`，经 `github.com/jackz-jones/common/db` 建立连接）
- 代码生成：无 goctl model 生成；手写 model（`internal/store/model.go`）+ Repository 接口（`internal/store/repository.go`）

### 现有表清单（`internal/store/model.go`，共 10 张）

| 表 | 实体 | 关键索引 |
|----|------|---------|
| tenants | Tenant | name/email 唯一 |
| users | User | tenant_id |
| api_keys | APIKey | tenant_id / user_id |
| tenant_chain_configs | TenantChainConfig | 复合 `idx_tenant_chain`(tenant_id, chain_name) |
| tenant_chain_nodes | TenantChainNode | chain_config_id |
| tenant_contract_configs | TenantContractConfig | tenant_id / chain_config_id |
| call_logs | CallLog | 复合 `idx_calllog_tenant_created`、`idx_calllog_tenant_status_method` + tx_id 单列 |
| bills | Bill | 复合 `idx_bill_tenant_period` |
| quotas | Quota | tenant_id |
| audit_logs | AuditLog | 复合 `idx_auditlog_tenant_created`、`idx_auditlog_user_created` + action |

> `DailyUsageStat` 为非表结构体（聚合查询投影）。多个 `gorm:"-"` 虚拟字段用于 JSON 输出（TenantName / KeyMasked 等）。

### ORM 禁忌

| # | 禁止 | 原因 | 替代 |
|---|------|------|------|
| 1 | `SELECT *` | 无关字段浪费 IO、字段变动易出问题 | 显式列名 |
| 2 | `.Save()` 用于更新（GORM） | 语义模糊，会触发不必要的 UPDATE | `.Updates(...)` 或 `.Update(...)` |
| 3 | N+1 查询 | 性能极差 | `Preload` / `JOIN` |
| 4 | 无 `WHERE` 的 `UPDATE` / `DELETE` | 全表灾难 | 必须带主键或唯一条件 |
| 5 | 字符串拼接 SQL | SQL 注入 | 参数化查询 |
| 6 | 在循环中查 DB | N 次往返 | 批量查询 + 内存合并 |

### 事务
- 使用 `db.Transaction(...)` 包裹多写操作（现有用法见 `repository.go` 的 `ReplaceChainNodes`）
- 必须**检查事务函数的返回值**，错误时框架自动回滚
- 事务内**禁止**调用外部接口（容易长事务锁表）
- 事务超时：默认 3 秒

### 缓存
- 本项目：DB 层无分布式缓存；进程内缓存为 `config_resolver.go` 的 `sync.Map`（链/合约配置 ID，按链配置精确失效）
- Redis 键：订阅断点经 `Get/SetLatestBlockHeight`（链维度）；事件推送走 Redis Stream
- 缓存空值（短 TTL，如 60s）防止穿透
- 必须设过期时间，**禁止**永久缓存（除非显式确认）

## 4. 性能与容量

| 项目 | 阈值 |
|------|------|
| 单表行数预警 | 500 万（超过考虑分表/归档） |
| 慢查询阈值 | ≥**500ms** 必须优化（生产 200ms 告警） |
| 连接池上限 | ≤**100**（按实例规格调整） |
| 查询超时 | 默认 3 秒，对外接口必设 |
| 批量操作分片 | ≤1000 行/批 |

## 5. 迁移（Migration）

- 工具：**双轨**——GORM AutoMigrate（`DatabaseConf.AutoMigrate` 开启时对 `internal/store/db.go allModels` 10 张表生效，适用于 dev/test）+ 手工 SQL（`migrations/`，供 DBA 生产执行）
- 文件命名：`YYYYMMDD_{{描述}}.sql`，例：`migrations/20260731_add_query_indexes.sql`（5 个 `CREATE INDEX IF NOT EXISTS` 复合索引）
- 生产环境**禁止**开启 AutoMigrate，以 `migrations/` SQL 为准（历史踩坑：AutoMigrate 触发 MySQL Error 1091，见 failures.md `e362798c`）
- 必须可回滚：破坏性变更（drop/rename/类型变更）必须评审 + 灰度
- 保留完整迁移历史，**禁止**删除已发布的迁移文件

## 6. 备份与灾备

- 备份频率：<!-- TODO(sop.init) 按运维实际填写 -->
- 保留时长：<!-- TODO(sop.init) 按运维实际填写 -->
- 演练频率：<!-- TODO(sop.init) 按运维实际填写 -->

## 7. 数据安全

- **【必须】** 敏感字段加密存储：身份证、手机号、银行卡、密码等
- **【必须】** 加密算法见 [coding-style.md §安全编码](coding-style.md)
- **【必须】** 数据访问审计日志：变更类操作（INSERT/UPDATE/DELETE）必须可追溯
- **【必须】** 生产数据库**禁止**直连访问，必须经过堡垒机或受控通道
- **【必须】** 测试/开发环境数据脱敏，**禁止**复制生产真实数据

## 8. 数据库类型兼容

### 项目支持的数据库列表

本项目支持以下数据库引擎，所有 ORM 模型定义与迁移脚本**必须**在列表内的数据库上验证通过：

| # | 数据库 | 版本 | 用途 | 备注 |
|---|--------|------|------|------|
| 1 | MySQL | 8.0+ | 主业务数据 | `DatabaseConf.Type: mysql` |
| 2 | PostgreSQL | 14+ | 主业务数据 | helm 默认（`postgresql:5432`） |
| 3 | Kingbase | V8 | 主业务数据（信创） | `DatabaseConf.Type: kingbase` |

> 根据实际项目填写，删除不使用的行。如果项目仅使用单一数据库则保留一行即可。

### 字段类型兼容性约束

在创建或修改 ORM 模型时，**禁止**使用目标数据库不支持的字段类型。开发者必须遵守以下规则：

1. **【必须】** 仅使用项目所有目标数据库均支持的字段类型，不得依赖特定数据库独有类型
2. **【必须】** 新增字段前查阅下方兼容类型映射表，确认所有目标数据库均可映射
3. **【禁止】** 在 ORM 模型中使用数据库特有类型（如 MySQL 的 `ENUM`、PostgreSQL 的 `ARRAY` / `HSTORE` / `JSONB`），除非项目明确仅支持该单一数据库
4. **【禁止】** 使用未经团队评审的自定义类型或扩展类型

### 通用字段类型映射（推荐）

| 语义类型 | MySQL | PostgreSQL | Kingbase | TiDB | SQLite | ORM 推荐声明 |
|----------|-------|------------|----------|------|--------|--------------|
| 整数主键 | `BIGINT UNSIGNED` | `BIGINT` | `BIGINT` | `BIGINT` | `INTEGER` | 框架自动适配 |
| 字符串 | `VARCHAR(N)` | `VARCHAR(N)` | `VARCHAR(N)` | `VARCHAR(N)` | `TEXT` | `string` / `CharField(max_length=N)` |
| 长文本 | `TEXT` | `TEXT` | `TEXT` | `TEXT` | `TEXT` | `text` |
| 布尔 | `TINYINT(1)` | `BOOLEAN` | `BOOLEAN` | `TINYINT(1)` | `INTEGER` | `bool` |
| 时间戳 | `DATETIME(3)` | `TIMESTAMP(3)` | `TIMESTAMP(3)` | `DATETIME(3)` | `TEXT` (ISO8601) | `datetime` |
| 精确小数 | `DECIMAL(M,N)` | `NUMERIC(M,N)` | `NUMERIC(M,N)` | `DECIMAL(M,N)` | `REAL` | `decimal` |
| JSON 数据 | `JSON` | `JSONB` | `JSONB` | `JSON` | `TEXT` | 框架 JSON 序列化 |
| UUID | `CHAR(36)` / `BINARY(16)` | `UUID` | `UUID` | `CHAR(36)` / `BINARY(16)` | `TEXT` | 视框架支持选择 |
| 二进制 | `BLOB` | `BYTEA` | `BYTEA` | `BLOB` | `BLOB` | `bytes` / `binary` |

> 如项目仅支持单一数据库，可使用该数据库特有类型，但需在表头注明。

### ORM 模型审查要点

代码审查（CR）时需确认：

- [ ] 模型字段类型是否在上述兼容映射表内
- [ ] 是否存在依赖特定数据库行为的默认值表达式（如 MySQL `NOW(3)` vs PostgreSQL `now()`）
- [ ] 迁移脚本是否能在所有目标数据库上执行成功
- [ ] 若确需使用数据库特有类型，是否已在模型注释中标注原因并经评审同意

## 参考

### 项目文档

- 编码与安全：[coding-style.md](coding-style.md)
- 上下游依赖：[relationship.md](../relationship.md)
- API 规范：[api-standards.md](api-standards.md)
- 部署规范：[deployment.md](deployment.md)

### 数据库官方文档

- MySQL：<https://dev.mysql.com/doc/refman/8.0/en/data-types.html>
- PostgreSQL：<https://www.postgresql.org/docs/current/datatype.html>
- Kingbase（人大金仓）：<https://help.kingbase.com.cn/v8/index.html>
- TiDB：<https://docs.pingcap.com/tidb/stable/data-type-overview>
- SQLite：<https://www.sqlite.org/datatype3.html>
