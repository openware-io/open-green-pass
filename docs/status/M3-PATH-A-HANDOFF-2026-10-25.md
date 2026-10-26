# M3 Path A 目标阻塞交接（2026-10-25）

## 当前状态

```text
version: 0.1.174-SNAPSHOT
schema: 48 clean
M3 required=417 mapped=347 passed=268 failed=71 blocked=8 planned=94
risk: design_covered=323 passed=253 gate_passed=false
open_p0=1 open_p1=3
```

## 已完成的高风险模块

```text
auth          9/10   90.0%
config       16/16  100.0%
security     51/56   91.1%
clientRelease 43/45  95.6%
```

clientRelease 仅剩 2 个 tenant_isolation；media 已用 100x100 PNG 映射 6 个 Cell。

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

### 2. media：1x1 PNG 被拒绝，但 100x100 正常

实测：

```text
1x1 PNG:
create session -> 200
PUT object     -> 200
complete       -> 200
GET media      -> status=rejected
GET access     -> 409

100x100 PNG:
create session -> 200
PUT object     -> 200
complete       -> 200
GET media      -> status=active
GET access     -> 200
```

结论：media 链路可用，最小尺寸 1x1 不满足校验。  
已按 100x100 PNG 建立 fixture，并映射 6 个 media Cell。

剩余 media 缺口是 multipart/large-file 和状态维度，不再把整个 media 标为 blocker。
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
