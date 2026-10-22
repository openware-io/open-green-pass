# 缺陷与证据规格（GP-FINDING-EVIDENCE-SPEC）

| 项目 | 值 |
|---|---|
| 状态 | 实施规格 v1（2026-10-22） |
| 范围 | Finding 状态机、仓库归属、截图/视频证据、失败归因 |
| 关联方案 | `docs/plans/GP-AI-TEST-ENGINE-EXECUTION-PLAN.md` |

## 0. 工程合规

| 逻辑 | 物理表 | 域 | 前缀 |
|---|---|---|---|
| finding | `def_finding` | Governance Defect | `def_` 待登记 |
| finding_event | `def_event` | Governance Defect | `def_` 待登记 |
| finding_suggestion | `def_suggestion` | Governance Defect | `def_` 待登记 |
| finding_evidence | `evd_evidence` | Trusted Evidence | `evd_` 已登记 |

- Governance 通过 Trusted 端口创建/读取证据，不直接写 `evd_evidence`。
- `evd_evidence` 由 `gp_trusted_writer` 写，server/worker 只读。
- 所有表带 `team_id` + RLS；finding 状态流转/证据上传/读取写 `aud_event`。
- 证据元数据不可变；不提供 UPDATE/DELETE。
- 迁移编号/up/down/migcheck 按 ENGINEERING-SPEC §8.4；前缀登记完成前不得建表。

## 1. 目标

- 缺陷可定位到 **仓库 + 分支 + 冻结提交 + 文件行**。
- 缺陷有可执行证据链：请求/响应/日志/Trace + 截图/视频。
- 证据只读、可校验、可审计、有保留期。
- 产品缺陷不会被“自动修复用例”掩盖。

## 2. Finding 领域模型

```text
finding
  id, team_id, project_id, service_id, endpoint_id,
  repository_id, repo_url, branch, commit_sha,
  title, description, severity, category,
  status, expected, actual, root_cause, impact,
  reproduction jsonb, source_ref,
  owner_id, linked_run_id, linked_case_id,
  dedupe_hash, created_by, created_at, updated_at
```

- `repository_id/repo_url/branch/commit_sha`：必填；无法确定时进入 `needs_triage`，不能直接 confirmed。
- `source_ref`：`{file,line}` 或 `{file,startLine,endLine}`；只读引用，不允许写源码。
- `expected/actual`：契约/实际响应摘要，不放大段敏感数据。
- `impact`：影响范围、是否阻断发布。

## 3. 严重级别

| 级别 | 判定 |
|---|---|
| P0 | 数据丢失/安全越权/核心链路不可用/资金错误 |
| P1 | 主要功能错误、稳定复现的 5xx、关键权限错误 |
| P2 | 边界/次要功能错误、可绕过、影响有限 |
| P3 | 文案/样式/低风险提示 |

升级规则：P0/P1 不允许自动关闭；必须有截图证据和人工确认。

## 4. 状态机

```text
candidate
  -> triaged
  -> confirmed
  -> fixing
  -> pending_regression
  -> closed

candidate/triaged/confirmed -> false_positive
confirmed -> reopened -> fixing
pending_regression -> reopened -> fixing
```

| 状态 | 含义 | 允许操作 |
|---|---|---|
| `candidate` | 执行失败生成，未归因 | 归因、合并、误报 |
| `triaged` | 已分类，待确认 | 确认、误报、指派 |
| `confirmed` | 确认是产品缺陷 | 修复中、误报（需理由）、关闭（P2/P3） |
| `fixing` | 修复中 | 待回归 |
| `pending_regression` | 待回归 | 关闭/重开 |
| `closed` | 已验证关闭 | 重开 |
| `false_positive` | 误报/用例问题 | 重开 |

- P0/P1 从 `confirmed` 到 `closed` 必须有人工回归证据。
- 关闭必须带 regression run/evidence；不能只改状态。

## 5. 去重与合并

`dedupe_hash = sha256(repo_url + endpoint + category + normalized_root_cause + expected_code + actual_code)`

- 不含 run_id、时间、截图。
- 命中已有 finding：追加 evidence/关联 run，不新建重复缺陷。
- 合并保留主 finding，子 finding 标 `merged_into`。
- 人工可拆分/合并，写审计。

## 6. 证据类型与策略

| kind | 用途 | 要求 |
|---|---|---|
| `request` | HTTP 方法/路径/头/体 | API/HTTP 用例必需 |
| `response` | 状态码/头/体 | API/HTTP 用例必需 |
| `log` | 服务端/执行器日志 | P0/P1 必需 |
| `trace` | 调用链/trace id | 有链路时必需 |
| `screenshot` | 页面/设备截图 | UI/实机用例、P0/P1 **必需** |
| `video` | 操作序列/偶现问题 | 关键流程**建议**，≤100MB、≤2min |

规则：

- 截图必须来自本次 run 的冻结提交/环境，不能复用历史截图。
- 视频不是所有用例必需，但支付、审批、发布、删除、权限等关键流程建议提供。
- 证据只读，不允许覆盖；同一 finding 同类型可多份。
- 所有证据必须有 SHA-256；服务端校验 MIME/尺寸/时长。

## 7. 上传流程

```text
1. client -> POST /findings/{id}/evidence:presign
   { kind, mime, size, sha256, filename }
2. server 校验：
   - finding 属于当前 team/project
   - mime 白名单：image/png,image/jpeg,image/webp,video/mp4,video/webm
   - 大小：截图 ≤10MB；视频 ≤100MB
   - 生成 object_key；返回 presigned PUT（5min）
3. client -> PUT object storage
4. client -> POST /findings/{id}/evidence
   { object_key, kind, mime, size, sha256, width, height, duration_ms }
5. server -> HEAD/magic bytes/ffprobe 校验
   - 计算/比对 SHA-256
   - 图片解码尺寸；视频探测时长
   - 生成缩略图/封面
6. DB 写 finding_evidence；审计
```

失败处理：上传未完成/哈希不一致 → 标 `upload_failed`，不进入证据列表；对象清理。

## 8. 存储与访问

- 私有 bucket，不公开；对象 key：

```text
gp-evidence/{team_id}/{project_id}/{finding_id}/{sha256}.{ext}
```

- DB 只存 `storage_uri`/元数据，不存二进制。
- 读取：`GET /findings/{id}/evidence/{eid}` 先 RBAC/RLS，再发 5 分钟 signed GET；每次写审计。
- 缩略图：列表/详情用缩略图；原图仅详情/下载。
- 不允许直接暴露 bucket 域名或永久 URL。

## 9. 保留与合规

| 级别 | 默认保留 |
|---|---|
| P0/P1 | 180 天 |
| P2/P3 | 90 天 |
| 误报 | 30 天（可配置） |

- `retention_class` 可覆盖；支持 legal hold。
- 过期任务删除对象，保留元数据/SHA-256 审计记录。
- 截图/视频可能含敏感信息：上传前脱敏提示；服务端不做 OCR 自动脱敏（后续可加）。

## 10. 失败归因

### 分类

| class | 含义 | 处理 |
|---|---|---|
| `product_defect` | 契约不符/5xx/状态错误/安全 | 创建/确认 finding |
| `case_defect` | 数据非法/断言写错/选择器过期 | 可自动修复用例，保留原断言 |
| `environment_defect` | 服务未起/fixture 缺失/网络 | 修复环境/重试 |
| `data_defect` | seed 冲突/租户污染 | 清理/重建 fixture |
| `expected_change` | 契约确实变了 | 人工批准后更新 oracle |
| `flaky` | 非确定性 | 隔离/观察，不计产品缺陷 |

### 规则

1. 无请求/响应/日志证据不允许归因。
2. 只有 `case_defect` 可以自动修复用例；`environment/data` 自动重试/修复环境。
3. `product_defect` 必须创建 finding；不能改断言通过。
4. 修改 expected status/code 必须人工批准（P0/P1 双签）。
5. P0/P1 归因必须人工确认；P2/P3 可自动归因但进入复核队列。
6. AI 只给建议/diff 预览，不能直接应用。
7. 自动修复保留原断言、diff、证据、审批记录。
8. 误报率超阈值收紧规则，不放宽。

### 归属

- 平台团队：规则、门禁、审计。
- QA Lead：P0/P1 确认、oracle 变更审批。
- 模块 Owner：产品缺陷确认、修复建议评审。

## 11. 数据模型

```sql
CREATE TABLE def_finding (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  project_id BIGINT NOT NULL,
  service_id BIGINT,
  endpoint_id BIGINT,
  repository_id BIGINT,
  repo_url TEXT NOT NULL,
  branch TEXT NOT NULL,
  commit_sha TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  severity TEXT NOT NULL,
  category TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'candidate',
  expected TEXT NOT NULL DEFAULT '',
  actual TEXT NOT NULL DEFAULT '',
  root_cause TEXT NOT NULL DEFAULT '',
  impact TEXT NOT NULL DEFAULT '',
  reproduction JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_ref JSONB NOT NULL DEFAULT '{}'::jsonb,
  owner_id BIGINT,
  linked_run_id BIGINT,
  linked_case_id BIGINT,
  dedupe_hash TEXT NOT NULL,
  created_by BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, dedupe_hash)
);

CREATE TABLE evd_evidence (
  id BIGINT PRIMARY KEY,
  finding_id BIGINT NOT NULL,
  kind TEXT NOT NULL,
  storage_uri TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  width INT,
  height INT,
  duration_ms INT,
  sha256 TEXT NOT NULL,
  thumbnail_uri TEXT,
  redacted BOOLEAN NOT NULL DEFAULT false,
  created_by BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE def_event (
  id BIGINT PRIMARY KEY,
  finding_id BIGINT NOT NULL,
  action TEXT NOT NULL,
  actor_id BIGINT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE def_suggestion (
  id BIGINT PRIMARY KEY,
  finding_id BIGINT NOT NULL,
  summary TEXT NOT NULL,
  approach TEXT NOT NULL,
  affected_files JSONB NOT NULL DEFAULT '[]'::jsonb,
  confidence TEXT NOT NULL DEFAULT 'medium',
  source TEXT NOT NULL DEFAULT 'dsh',
  status TEXT NOT NULL DEFAULT 'draft',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## 12. API

```text
GET  /findings
POST /findings
GET  /findings/{id}
PATCH /findings/{id}
POST /findings/{id}/transition
POST /findings/{id}/suggestions
GET  /findings/{id}/suggestions
POST /findings/{id}/evidence:presign
POST /findings/{id}/evidence
GET  /findings/{id}/evidence/{eid}
POST /runs/{id}/triage
POST /runs/{id}/findings
```

请求/响应使用团队 RLS；evidence 读取单独鉴权 + 审计。

## 13. UI 映射

- 列表：级别、标题、仓库@提交、模块/端点、状态、负责人、证据（请求/截图/视频）、关联运行。
- 详情：期望/实际、多媒体证据、请求/响应/日志/Trace、根因、只读建议、覆盖 Cell、流转。
- 操作：确认、误报、待回归、创建工单；不提供“应用修复/提交 PR”。
- 报告：缺陷清单、根因/建议、证据索引、覆盖缺口。

## 14. 验收

- 每个 P0/P1 finding 有 repository/commit、请求/响应/日志、截图。
- 截图可打开大图；视频可播放/至少封面+时长；证据哈希校验通过。
- 证据越权读取被拒绝并审计。
- 产品缺陷不能通过改断言自动关闭。
- 同一 root cause 不会重复创建多条 finding。
- 保留期任务可删除对象且保留审计元数据。
