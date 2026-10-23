# M3：user 模块执行覆盖（2026-10-23）

## 执行

run `2906443507273728`（旧 runner 兼容前）和 `2906703985086464`（兼容后）：

- 场景 1
- 13 个 user 用例
- target `im-saas`
- env `test`
- target version `2.4.0-SNAPSHOT`

结果：

```text
pass 1（user-admin-list-live-001）
fail 12（旧格式兼容后实际发出请求；2 条通过，10 条因 `{id}` 占位符返回 HTTP 500）
```

## 覆盖回写

`POST /runs/2906443507273728/coverage-sync`：

```text
updated=13
```

当前 user 模块：

```text
total_required=24
design_covered=24
executed=24
execution_coverage=1.0
open_p0=0
open_p1=2
gate_passed=false
blocker=open P0/P1 findings: 0/2
```

## 问题

1. 12 条 `user-gen-*` 仍是旧扁平脚本，但 runner 已兼容并解析 BaseURL：

```json
{"path": "/api/v1/admin/users", "method": "GET", "expected_status": 200}
```

执行器要求：

```json
{"request": {...}, "assertions": [...]}
```

2. 多条旧脚本含 `{id}` 占位符，且部分为破坏性 DELETE/PUT，不能直接自动执行；这是当前 10 条失败的根因。
3. 当前 “executed=24” 包含 10 条占位符 500，不代表 24 条真实有效通过；runner 已不再返回 `case has no declarative request`。
4. 项目门禁仍有：

```text
open_p0=1
open_p1=3
```

## 下一步

- 为旧 user 用例补全 declarative request；对破坏性操作使用专用测试数据或显式 skip/block。
- 为 24 个用户 Cell 建立可执行用例映射，而不是只用 13 条。
- 扩展到全部 18 个模块。
- 关闭已确认 P0/P1 或标记豁免。
