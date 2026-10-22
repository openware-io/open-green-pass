# 覆盖模型规格（GP-COVERAGE-MODEL-SPEC）

| 项目 | 值 |
|---|---|
| 状态 | 实施规格 v1（2026-10-22） |
| 范围 | 端点盘点、Coverage Cell、枚举/去重、覆盖门禁 |
| 关联方案 | `docs/plans/GP-AI-TEST-ENGINE-EXECUTION-PLAN.md` |

## 0. 工程合规

- 物理表名使用 `cov_cell`、`cov_waiver`；`cov_` 为待登记前缀，登记前不得创建迁移。
- 归属 Governance Coverage；所有表带 `team_id` + RLS；waiver 审批写 `aud_event`。
- 迁移编号、up/down、migcheck/testcontainers 按 ENGINEERING-SPEC §8.4。
- DDL 草案中的表名在实施时按上表替换；不得使用未登记的 `ai_` 前缀。

## 1. 目标

把“覆盖全面”变成可计算、可审计的模型：

- 每个端点有明确的必需覆盖维度；
- 每个维度有唯一 Cell；
- 每个 Cell 有用例或已批准 waiver；
- 覆盖度由程序计算，不依赖 Agent 自报。

## 2. 术语

| 术语 | 定义 |
|---|---|
| Endpoint | `method + path`，来自 OpenAPI/路由/controller |
| Archetype | 端点类型：list/detail/create/update/delete/action/upload/auth |
| Dimension | 测试意图：happy/authn/authz/validation/error/state/... |
| Equivalence Class | 某维度下的具体类，如 `required_missing`、`page_size=max+1` |
| Cell | 端点 × 维度 × 等价类 × 角色 × 状态 的最小覆盖单元 |
| Case | 一条可执行用例，声明覆盖的 `coverage_cell_ids` |
| Required Cell | 必须覆盖的 Cell；未覆盖必须 waiver |

## 3. Cell ID

```text
cell:<endpoint_hash>:<dimension>:<class_slug>:<role>:<state>
```

- `endpoint_hash`：`sha1(method + " " + normalized_path)[:12]`，避免路径特殊字符。
- `class_slug`：小写、`[a-z0-9_.-]`，最长 48。
- `role`：`anon|admin|operator|viewer|tenant-other|owner|non-owner|any`。
- `state`：`none` 或 `<from>-><to>`。

同时落库拆分字段，便于查询：

```text
id, team_id, project_id, service_id, endpoint_id,
endpoint_hash, method, path, dimension, equivalence_class,
role, state, required, priority, status, case_id,
waiver_id, created_at, updated_at
```

## 4. Endpoint Archetype

优先使用 OpenAPI/路由元数据；缺失时用启发式：

| Archetype | 判定 |
|---|---|
| `auth` | path 以 `/auth` 开头或 operation 标记 auth |
| `upload` | path 含 `upload|artifact|media|multipart` 或请求为 multipart |
| `state` | operation 标记 transition，或 path 含 `submit|approve|reject|revoke|rollout|pause|resume|withdraw|archive` |
| `list` | GET 且无 path 参数，返回集合 |
| `detail` | GET 且有 path 参数 |
| `create` | POST 且无 path 参数 |
| `action` | POST 且有 path 参数（非 upload/state） |
| `update` | PUT/PATCH |
| `delete` | DELETE |

一个端点可有多个 archetype（如 `action+state`），按并集取必需维度。

## 5. 必需维度矩阵

| Archetype | 必需维度 |
|---|---|
| `list` | happy、authn、authz、pagination、empty、invalid_filter、sort |
| `detail` | happy、not_found、authz、invalid_id、tenant_isolation |
| `create` | happy、required_missing、type_format、enum、duplicate_conflict、authz、side_effect |
| `update` | happy、not_found、partial_invalid、stale_version、authz、state_transition |
| `delete` | happy、not_found、repeat_idempotent、authz、referential_conflict、confirm_mismatch |
| `action` | happy、not_found、invalid_state、authz、idempotency、side_effect |
| `state` | allowed_transition、forbidden_transition、concurrent_transition、authz |
| `upload` | happy、size_limit、mime_limit、presign_expiry、checksum_mismatch、cancel_resume、authz |
| `auth` | success、bad_credentials、lockout_rate_limit、expired_token、reset_invalid/expired |

### 参数级

每个参数至少生成：

- `required_missing`（如必填）
- `type_format`
- `enum`（如有）
- `boundary`（min-1/min/max/max+1，数值/字符串长度）
- `null_empty`

参数级 Cell 上限：每个端点 12 个参数 Cell；超过按风险截断，并记录 waiver/gap。

### P0 加严

P0 端点额外要求：

- `authz` 至少 2 个角色/租户组合；
- 写操作 `idempotency`/`concurrency`；
- `data_consistency`（创建→读取→更新→删除）。

## 6. 枚举算法

```text
for endpoint in inventory:
  archetypes = detect(endpoint)
  dims = union(required_matrix[archetype])
  for dim in dims:
    classes = equivalence_classes(endpoint, dim)
    for cls in classes:
      add cell(endpoint, dim, cls, role=any, state=none, required=true)
  if endpoint.has_authz:
    add role variants for authz
  if endpoint.state_machine:
    add allowed/forbidden transitions
  if endpoint.upload:
    add upload classes

# 参数级
for param in endpoint.parameters:
  add required/type/enum/boundary/null cells

# pairwise 组合（多参数）
combos = pairwise(param classes) capped 20/endpoint
add cells(dimension=validation_combo, required=true if P0 else false)
```

去重键：

```text
team_id + endpoint_hash + dimension + class_slug + role + state
```

## 7. Required / Optional / Waiver

- `required=true`：矩阵必需、P0 加严、参数必需 Cell。
- `required=false`：pairwise 扩展、非关键边界、性能/安全附加。
- waiver 条件：
  - 端点不可达/已废弃；
  - 环境无法提供（如无预签名桶）；
  - 风险接受。
- waiver 字段：`reason`、`owner`、`expires_at`、`approved_by`、`evidence`。
- 过期 waiver 自动失效，回到未覆盖。

## 8. 覆盖率计算

```text
required = cells where required and waiver not active
design_covered = required where case_id is not null and status in (generated, executed, passed)
executed = required where status in (executed, passed)
passed = required where status = passed
blocked = required where status = blocked
```

指标：

- `design_coverage = design_covered / required`
- `execution_coverage = executed / required`
- `pass_coverage = passed / required`
- `waiver_rate = waived / all required`
- `uncovered = required - design_covered`

门禁：

- `design_coverage = 100%`（含 waiver 后仍为 100%）
- `execution_coverage >= 90%`
- P0 `pass_coverage >= 95%`，且无未豁免 failed
- 任何 `blocked` 必须有原因和 owner
- 报告必须同时展示 design/execution/pass，不能只报“通过率”

## 9. 示例：`open-chat-admin` 2.2.1

盘点：18 模块、87 端点（81 Bearer、4 public、2 presigned）。

估算：

- 87 端点 × 平均 6-9 个必需 Cell ≈ 520-780 个 Cell；
- pairwise/去重后约 500-700 个；
- 按 1.5-2 条 Cell/用例合并，约 300-500 条用例。

`/admin/users` 模块示例：

| 端点 | Archetype | 必需 Cell（示例） |
|---|---|---|
| `GET /admin/users` | list | happy、authn、authz、pagination、empty、invalid_filter、sort = 7 |
| `GET /admin/users/{id}` | detail | happy、404、authz、invalid_id、tenant = 5 |
| `PUT /admin/users/{id}/status` | update | happy、404、invalid_enum、authz、stale = 5 |
| `DELETE /admin/users/{id}` | delete | happy、404、wrong_confirm、admin_protected、authz = 5 |

仅 `user` 模块约 22 个必需 Cell；全项目 500-700 个 Cell 是合理量级。

## 10. 数据模型

```sql
CREATE TABLE cov_cell (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  project_id BIGINT NOT NULL,
  service_id BIGINT,
  endpoint_id BIGINT NOT NULL,
  endpoint_hash TEXT NOT NULL,
  method TEXT NOT NULL,
  path TEXT NOT NULL,
  dimension TEXT NOT NULL,
  equivalence_class TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'any',
  state TEXT NOT NULL DEFAULT 'none',
  required BOOLEAN NOT NULL DEFAULT true,
  priority TEXT NOT NULL DEFAULT 'P1',
  status TEXT NOT NULL DEFAULT 'planned',
  case_id BIGINT,
  waiver_id BIGINT,
  evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, endpoint_hash, dimension, equivalence_class, role, state)
);

CREATE TABLE cov_waiver (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  cell_id BIGINT NOT NULL,
  reason TEXT NOT NULL,
  owner_id BIGINT NOT NULL,
  approved_by BIGINT,
  expires_at TIMESTAMPTZ,
  evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`status` 枚举：`planned|generated|executed|passed|failed|blocked|waived`。

## 11. API

```text
POST /ai/coverage/plan
  { project_id, service_id?, module?, endpoint_ids? }
  -> { plan_id, cells, groups, gaps }

GET /ai/coverage/cells?project_id=&service_id=&status=
GET /ai/coverage/summary?project_id=
POST /ai/coverage/waivers
  { cell_id, reason, owner_id, expires_at? }
```

所有接口使用团队 RLS 和审计；生成/执行侧只能更新 `status/case_id/evidence`。

## 12. 与生成/执行集成

- 生成前：Planner 产出 Cell；Generator 只针对分配的 Cell 生成用例。
- 用例入库：必须带 `coverage_cell_ids`；校验 Cell 属于同模块。
- 执行：用例结果回写 Cell 状态（executed/passed/failed/blocked）。
- 门禁：读取 summary，不信任 Agent 汇总。

## 13. 验收

- 同一 inventory 两次 plan 得到相同 Cell 集合（稳定 ID）。
- 每个端点至少包含 archetype 矩阵的必需维度。
- 每个参数有 required/type/boundary 或明确 waiver。
- summary 可复算；Design/Execution/Pass 分开。
- `open-chat-admin` plan 产出 500-700 Cell，无重复。
- 100% required Cell 有 case 或有效 waiver 才算门禁通过。
