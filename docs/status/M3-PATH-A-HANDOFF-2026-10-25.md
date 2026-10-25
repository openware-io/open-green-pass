# M3 Path A 目标阻塞交接（2026-10-25）

## 当前状态

```text
version: 0.1.174-SNAPSHOT
schema: 48 clean
M3 required=417 mapped=341 passed=262 failed=71 blocked=8 planned=100
risk: design_covered=317 passed=247 gate_passed=false
open_p0=1 open_p1=3
```

## 已完成的高风险模块

```text
auth          9/10   90.0%
config       16/16  100.0%
security     51/56   91.1%
clientRelease 43/45  95.6%
```

clientRelease 仅剩 2 个 tenant_isolation。

## 目标侧阻塞

### 1. openPlatform POST 405

```text
POST /api/v1/admin/open-platform/applications
405 METHOD_NOT_ALLOWED
```

导致 3 个 Cell 无法执行，模块 11/14 = 78.6%。

修复要求：

```text
提供 POST 创建应用路由，或明确该能力不在当前版本范围并给出排除决定。
```

### 2. media 校验链路 rejected

使用合法 1x1 PNG：

```text
create upload-session -> 200
PUT object             -> 200
complete               -> 200
GET /media/{id}        -> status=rejected
GET /media/{id}/access -> 409
```

说明媒体校验/异步处理链路不可用，不是缺 fixture。

修复要求：

```text
确认 media 校验器/处理器为何 rejected，并让合法 PNG 最终变为 active；
否则 media 41 个 Cell 只能 blocked。
```

### 3. Open P0/P1

```text
P0 2903529658814464 payment
   支付回调验签失败仍返回 500，重复回调可能造成订单状态不一致

P1 2903531475767296 user
   删除用户时 confirmUsername 不一致仍执行删除

P1 2903531478683648 clientRelease
   客户端发布提交使用过期 rowVersion 返回 500 而非 409

P1 2903748672557056 user
   user list returned 500
```

这些必须由目标 owner 修复或书面豁免。

### 4. 其他 fixture

```text
state/冲突 ~48 Cell：需要可创建/删除专用用户、被引用资源、可 reset 状态
tenant_isolation 4+2：需要第二租户和第二租户 token
```

## 可复现探针

新增：

```text
scripts/dev/probe-target-blockers.ps1
```

运行：

```powershell
pwsh -NoProfile -File scripts/dev/probe-target-blockers.ps1
```

当前输出：

```text
openPlatform 405
media complete 200 -> status rejected -> access 409
clientRelease 201（使用 UUID Idempotency-Key）
```

注意：clientRelease 创建本身可用。此前 500 是**非 UUID Idempotency-Key** 导致，已作为误报关闭，并另建 P2：

```text
2910814262497280 非 UUID Idempotency-Key 返回 500
```

## Path A 完成条件

```text
1. openPlatform POST 405 修复
2. media 合法 PNG 变 active
3. P0=0、P1=0 或有正式豁免
4. 提供可清理用户/状态数据/第二租户
```

满足后可按现有 risk 门禁和 strict 门禁重新执行，不需要改测试口径。
