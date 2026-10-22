# 垂直切片计划：user 模块（GP-VERTICAL-SLICE-1）

| 项目 | 值 |
|---|---|
| 状态 | 实施计划 v1（2026-10-22） |
| 目标 | 用最小闭环证明“覆盖 Cell → DSH 生成 → 执行 → 缺陷/证据 → 门禁” |
| 模块 | `open-chat-admin` 2.2.1 / `user` |
| 周期 | 5-7 个工作日 |
| 依赖 | M0 迁移/RLS/OpenAPI、`gp-dsh-gateway`、fixture runner |

## 1. 为什么选 user

- 有列表、详情、更新、删除四类端点，覆盖 archetype 多；
- 有权限/枚举/确认字段/404/自删等典型错误；
- 不依赖预签名对象存储，适合先打通；
- 业务价值明确：管理员用户管理是后台核心。

端点：

```text
GET    /api/v1/admin/users
GET    /api/v1/admin/users/{id}
PUT    /api/v1/admin/users/{id}/status
DELETE /api/v1/admin/users/{id}
```

## 2. 范围

### 包含

- 4 个端点的契约盘点；
- 约 22 个必需 Coverage Cell；
- 12-16 条 API 用例（按 Cell 合并）；
- DSH SDK 单模块多步 session（规划 → 生成 → 校验）；
- 执行 1 轮，采集请求/响应/日志/Trace；
- 至少 1 条产品缺陷 + 截图 + 只读建议；
- 覆盖/缺陷门禁；
- 一场 15 分钟演示。

### 不包含

- 全项目 18 模块；
- UI/移动 runner；
- 自动修复产品源码；
- Jira/飞书工单；
- 视频证据（截图优先）；
- 性能/安全专项。

## 3. Coverage Cells（约 22）

| 端点 | 必需 Cell |
|---|---|
| `GET /users` | happy、authn、authz、pagination、empty、invalid_filter、sort |
| `GET /users/{id}` | happy、404、authz、invalid_id、tenant_isolation |
| `PUT /users/{id}/status` | happy、404、invalid_enum、authz、stale/重复 |
| `DELETE /users/{id}` | happy、404、wrong_confirm、admin_protected、authz |

参数级补充：`page/pageSize` 边界、`status` 枚举、`confirmUsername` 缺失/不一致。

## 4. 用例与执行

- 12-16 条用例，按 Cell 合并；
- 每条用例声明 `coverage_cell_ids`；
- 断言至少：status + `items/total` 或业务 code；
- fixture：
  - `gp-run-<id>` 租户；
  - admin/operator/viewer 三个账号；
  - 3 个测试用户（active/disabled/待删）；
- 执行环境：test 环境，版本校验通过；
- 证据：请求/响应/日志；失败用例截图。

## 5. DSH 任务

```text
job: gp-slice1-user
profile: gp-test-engine
workspace: open-chat-admin@a4487643 (read-only)
steps:
  1. 规划：读取 4 端点 + 22 Cell，输出覆盖计划
  2. 生成：按模块生成 12-16 条用例 + coverage_cell_ids
  3. 校验：静态校验 + Critic 去重/断言检查
  4. 修复：仅修 case_defect，不改产品源码
```

## 6. 交付物

| 交付 | 验收 |
|---|---|
| 契约盘点 | 4/4 端点、参数/错误/角色 |
| Coverage Cells | 22 个，required 100% 有 case 或 waiver |
| 用例 | 12-16 条，静态校验通过 |
| 执行 | 执行率 ≥90%，失败有归因 |
| 缺陷 | ≥1 条产品缺陷，含请求/响应/日志/截图/仓库@提交 |
| 建议 | 根因 + 只读修复建议 + 回归建议 |
| 门禁 | design 100%、execution ≥90%、P0/P1 证据齐全 |
| 源码写 | 0 |

## 7. 日程

| 天 | 工作 |
|---|---|
| D1 | 迁移 34-37、RLS、OpenAPI `/findings`/`/coverage`、生成类型 |
| D2 | Coverage plan + user 模块 Cell 入库 |
| D3 | gateway job + DSH 多步 session + 用例入库 |
| D4 | fixture + 执行 + 证据采集 |
| D5 | 失败归因 + finding + 截图 + 建议 |
| D6 | 门禁 + 报告 + 演示脚本 |
| D7 | 修复问题/补测试/发布记录 |

## 8. 演示脚本（15 分钟）

1. 打开缺陷中心，说明源码只读/仓库@提交。
2. 打开 user 模块覆盖计划：4 端点、22 Cell。
3. 触发 DSH 生成，展示 job 状态/session 事件。
4. 展示 12-16 条用例及 coverage_cell_ids。
5. 执行，打开一条失败：请求/响应/日志/截图。
6. 归因为产品缺陷，创建 finding，展示只读建议。
7. 打开门禁：覆盖/缺陷状态。
8. 强调：没有修改源码、没有提交 PR。

## 9. 风险

| 风险 | 应对 |
|---|---|
| DSH 输出不稳定 | Schema + 静态校验 + Critic；失败重跑 |
| fixture 污染 | 独立命名空间 + TTL + 清理验证 |
| 测试环境不可用 | 提前 health check；blocked 不算产品失败 |
| 截图缺失 | P0/P1 强制；执行器自动抓图 |
| 范围膨胀 | 只做 user 模块；其他模块后续复用 |

## 10. 完成定义

- `make check` 通过；
- 迁移/RLS/testcontainers 通过；
- gateway 取消/超时/清理测试通过；
- 演示脚本可重复执行；
- 发布记录含 commit/镜像/迁移/readiness/IM 核对；
- 通过后才进入第二模块（建议 `clientRelease` 或 `payment`）。
