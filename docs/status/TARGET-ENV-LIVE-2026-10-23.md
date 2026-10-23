# 测试环境参数 + im-saas 真实执行（2026-10-23）

## 新增功能页面

- 路由：`/target-env`
- 入口：被测对象页「测试环境」按钮
- 能力：
  - 选择被测对象
  - 环境（test/staging/prod）
  - 运行版本
  - Base URL
  - 凭证引用（不保存明文）
  - 非敏感确认值（tenant_id、admin_username、test_user_prefix）
  - 测试数据 Profile / 浏览器 Profile
  - 环境参数需求（key/名称/必填/敏感/原因）
  - 确认后保存
- 查询 API：`GET /api/targets/{id}/env/runtime?env=test`
- 保存 API：`POST /api/targets/{id}/env/runtime`

## im-saas 已填写配置

| 字段 | 值 |
|---|---|
| 被测对象 | `2876690473259008`（im-saas） |
| 环境 | `test` |
| 运行版本 | `2.4.0-SNAPSHOT` |
| Base URL | `http://gateway.open-im-local.svc.cluster.local:3002` |
| 凭证引用 | `k8s://open-im-local/open-im-env` |
| tenant_id | `1` |
| admin_username | `admin` |
| 测试数据 Profile | `im-saas-users` |
| 浏览器 Profile | `admin-desktop` |

已通过 API 验证：保存 200，查询 200。

## 真实执行链路

- 新增 `HTTPRunner`（标准库，不依赖 K8s Job/curl 镜像）
- 部署配置：
  - `GP_RUNNER_TYPE=http`
  - `GP_EXECUTION_BEARER_TOKEN` 来自 `gp` namespace `gp-im-token`
- 真实用例：`user-admin-list-live-001`
  - `GET /api/v1/admin/users?dto.page=1&dto.pageSize=5`
  - 断言：status 200 + body 包含 `items`
- 真实运行：`2904774788288512`
  - 结果：`pass`
  - `http_status=200`
  - 证据 hash：`sha256:956c6611...`
  - 响应体包含真实 im-saas 用户列表

## 修复

- `run_case_result.failure_class` 空字符串违反 `ck_run_case_result_failure_class`
  - 改为空值写 NULL
- 这是真实执行 500 的根因

## 注意

- 曾错误重置 im-saas `admin` 密码；已从 `open_group.idaas_account` 恢复原哈希，并改用独立账号 `gp-e2e`。详见 `INCIDENT-ADMIN-PASSWORD-2026-10-23.md`。
- 已将真实用例绑定到 `cov_cell 2903566203719680`（GET list happy），`coverage-sync` 后该 Cell 为 `passed`。`executed` 仍为 2，因为该 Cell 原先是 2 个失败 Cell 之一，属于替换而非净增。
