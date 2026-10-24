# 覆盖率缺口报告 API（0.1.163-SNAPSHOT）

## 功能

```text
GET /coverage/gaps?project_id=<id>&module=<optional>
```

返回：

- `total`: required / mapped / passed / failed / blocked / unmapped
- `modules`: 按模块聚合
- `dimensions`: 按维度聚合

## 用途

- 前端/报告直接展示缺口分布，不再手写 SQL。
- 明确哪些缺口是 `authz`、`invalid_filter`、`upload fixture` 等类别。
- 与 waiver API 配合：先看缺口，再对不适用维度做显式 waiver。

## 验证

```text
HTTP 200
total.required=441
total.mapped=105
total.unmapped=336
modules=18
dimensions=33
```

## 版本

```text
0.1.163-SNAPSHOT
schema 48 clean
```
