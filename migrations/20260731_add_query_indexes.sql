-- =====================================================================
-- Migration: 20260731_add_query_indexes.sql
-- 目的：为高频分页/统计查询增加复合索引，降低慢查询数量
-- 说明：GORM AutoMigrate 已能自动创建（见 internal/store/model.go 的 tag），
--       本文件供生产环境 DBA 手工执行时参考。
-- =====================================================================

-- 调用日志：按租户 + 时间倒序分页
CREATE INDEX IF NOT EXISTS idx_calllog_tenant_created
    ON call_logs (tenant_id, created_at);

-- 调用日志：按租户 + 状态 + 方法类型统计（Invoke/Query 成功/失败）
CREATE INDEX IF NOT EXISTS idx_calllog_tenant_status_method
    ON call_logs (tenant_id, status, method_type);

-- 审计日志：按租户 + 时间倒序分页
CREATE INDEX IF NOT EXISTS idx_auditlog_tenant_created
    ON audit_logs (tenant_id, created_at);

-- 审计日志：按操作人 + 时间倒序分页
CREATE INDEX IF NOT EXISTS idx_auditlog_user_created
    ON audit_logs (user_id, created_at);

-- 账单：按租户 + 账期起点定位月/日账单
CREATE INDEX IF NOT EXISTS idx_bill_tenant_period
    ON bills (tenant_id, period_start);
