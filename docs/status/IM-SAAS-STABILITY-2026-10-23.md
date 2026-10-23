# im-saas 平台运营后台故障定位与修复（2026-10-23）

## 现象

- 平台运营后台“租户管理 / 酒店管理”等页面点击报错。
- `platform-order-service` 0/1，反复 CrashLoop。
- `platform-admin-service` 出现 MySQL 连接已关闭错误：
  `ConnectionIsClosedException: No operations allowed after connection closed`。

## 根因

### 1. platform-order-service Flyway V4 迁移失败

- `open_saas.flyway_schema_history_order` 中 version 4 `htl domain model` 为 `success=0`。
- 表已创建，但 `ALTER TABLE htl_booking` 未完成（`hotel_id` 等列不存在）。
- 应用启动时 Flyway 校验失败，服务退出，导致酒店/订单相关页面报错。

### 2. MySQL 连接接近上限

- `max_connections=200`，`Threads_connected` 约 171–181。
- `platform-admin-service` 的 `TenantContextFilter` 遇到连接被关闭，随机报错。

## 修复

1. 手工补齐 `htl_booking` 缺失列和索引。
2. 将 Flyway V4 历史记录改为成功，并修正 checksum 为当前镜像迁移的 checksum。
3. 将 MySQL `max_connections` 临时提升到 500。
4. 重启 `platform-admin-service`，清理旧连接。
5. `platform-order-service` 重新启动。

## 验证

| 检查 | 结果 |
|---|---|
| platform-order-service | 1/1 Running |
| platform-admin-service | 1/1 Running |
| `/admin/hotel/frontdesk/tasks` | 返回 403 权限错误，不再 500 |
| platform-order 启动日志 | Started，无迁移异常 |
| platform-admin 新日志 | 无 ConnectionIsClosed |
| MySQL 连接上限 | 500（运行时） |

## 说明与剩余风险

- 本次只修复测试环境数据/运行参数，未修改 im-saas 源码。
- `max_connections=500` 是运行时设置，MySQL 重启后会恢复；需要落到 my.cnf 或 Helm values。
- 连接数 171 仍然偏高，建议产品侧检查各服务连接池上限与泄漏。
- Flyway V4 失败是目标产品发布问题，应由 im-saas 修复迁移脚本/发布流程；本次是测试环境修复。
- SaaS 管理后台统一登录使用 im-user admin 返回 403（缺少平台角色），未用该账号做完整 UI 回归；需要平台运营角色账号。
