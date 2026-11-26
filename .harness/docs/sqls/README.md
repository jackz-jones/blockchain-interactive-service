# 数据库 / 持久化结构

> 集中存放数据库表结构 / Schema 定义文件，作为开发与 review 的权威依据。
> DDL 变更必须同步更新此处。

---

## 目录约定

```
sqls/
├── README.md                # 本文件
└── <db_name>/<table>.sql    # 或 <db_name>_FINAL_SCHEMA.md（按项目实际）
```

## 维护原则

- DDL 与线上一致；改表必须同步本目录
- 索引必记，新增查询条件前确认索引覆盖
- 字段必带 COMMENT，枚举字段列出全部可能值
- 任务模板 `02-plan.md` / `03-implement.md` 中已强制要求 DB 变更同步本目录

---

## 数据库清单

| DB | 用途 | 主要表 | 文档路径 |
|----|------|-------|---------|
| mysql / postgres / kingbase（三选一，`DatabaseConf.Type`） | 主业务数据 | tenants, users, api_keys, tenant_chain_configs, tenant_chain_nodes, tenant_contract_configs, call_logs, bills, quotas, audit_logs（共 10 张） | [database.md](database.md) |
| Redis | 事件推送 Stream / 订阅断点 KV | 非 GORM（`jackz-jones/common/event`） | [database.md](database.md) §1 |

> 表结构定义（GORM model + 索引 tag）权威来源：`internal/store/model.go`；生产 DDL 增量：`migrations/`。
