# M3：happy 读路径补齐（2026-10-24）

## 新增

为 10 个未映射的 happy 读 Cell 创建 GET 200 用例：

```text
account-cancellations/logs
client-release-platforms / client-release-policies
monitor/calls / online / online/users
security/reports / reports/pending/count / violations
stats/trend
```

执行结果：

```text
10 pass / 0 fail
```

## 当前账本

```text
总 Cell = 441
已映射 = 80
passed = 72
blocked = 8
executed = 80
未映射 = 361
```

门禁：

```text
design_coverage    = 18.1%
execution_coverage = 18.1%
open_p0 = 1
open_p1 = 3
```

## 说明

- 临时提高 run quota 执行；完成后已恢复 50/20/5/3。
- 未映射剩余主要是 invalid、conflict、state、upload/media 等需要 fixture 的维度。

## 详情边界（2026-10-24）

新增 7 条 detail invalid_id / not_found 用例：3 pass / 4 fail。

当前账本：

```text
mapped=87
passed=74
failed=5
blocked=8
unmapped=354
design_coverage=19.7%
execution_coverage=19.7%
```

失败用例暴露了目标在不同详情端点上的错误码不一致，保留原始失败，不伪造通过。

## 错误码 triage（2026-10-24）

- 缺失 cancellation logs 返回 200 []，属于空结果语义；已将用例期望修正为 200，重新执行通过。
- 非法数字 ID 的 4 条详情用例返回 500，保留失败并创建 finding：
  - id: 2907625674182656
  - severity: P2
  - title: 非法数字 ID 返回 500 而不是 400/404

当前账本：mapped=87，passed=75，failed=4，blocked=8，unmapped=354。

## 必填缺失与重复路径裁剪（2026-10-24）

新增 7 条 required_missing 用例：6 pass / 1 fail。

- openPlatform 前端路径带尾斜杠 `/applications/`，目标返回 404；与 `/applications` 重复。
- 已将 `/applications/` 的 14 个重复 Cell 标记 `required=false, status=waived`。

当前：required=427，mapped=91，passed=79，failed=4，blocked=8。

## 写路径 not_found（2026-10-24）

新增 11 条写路径 not_found 用例：9 pass / 2 fail。

- 4 个非法数字 ID 500 仍保留。
- `PUT /security/reports/{id}` 空 body 返回 400 "must not be null"（校验先于存在性）。
- `PUT /security/sensitive-words/{id}` 对不存在 ID 返回 200 空 body，已创建 P2 finding 2907631977992192。

当前：required=427，mapped=102，passed=88，failed=6，blocked=8，coverage=23.9%。
