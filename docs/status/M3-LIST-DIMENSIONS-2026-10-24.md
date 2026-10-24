# M3 列表维度覆盖（2026-10-24）

## 新增

为 87 个未映射的列表维度 Cell 创建用例：

```text
pagination  -> ?page=1&pageSize=1        expected 200
empty       -> ?page=999&pageSize=1      expected 200
sort        -> ?sort=created_at&order=desc expected 200
```

run `2909349117300736`：

```text
execute_http = 202
state = done
72 pass
15 fail
```

## 失败分类

```text
404 ROUTE_NOT_FOUND
400 REQUEST_PARAM_MISSING: typeId
```

这些是真实接口缺口/参数要求，保留失败，不改成通过。

## 当前 M3

```text
required = 417
mapped   = 253
passed   = 223
failed   = 22
blocked  = 8
planned  = 164
design_coverage = 60.7%
```

## 说明

- 本轮通过真实查询参数覆盖列表语义。
- 失败用例暴露了目标路由缺失和必填参数。
- 未把失败改成 pass。

## invalid_filter 追加（2026-10-24）

新增 29 条 `?page=0&pageSize=-1` 用例：

```text
1 pass
28 fail
```

目标返回 200，并把 `pageSize=-1` 回显，说明列表参数校验缺失。已创建 finding：

```text
2909355341385728  P2
列表接口接受 pageSize=-1 等非法分页参数
```

当前门禁：

```text
total_required = 417
design_covered = 258
executed = 258
design_coverage = 61.9%
execution_coverage = 61.9%
```
