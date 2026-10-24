# M3 适用性裁剪：singleton/aggregate 端点（2026-10-24）

## 规则

以下端点不是分页集合，规划器生成的以下维度不适用：

```text
GET /api/v1/admin/config
GET /api/v1/admin/stats/overview
GET /api/v1/admin/stats/trend
GET /api/v1/admin/monitor/server
GET /api/v1/admin/security/reports/pending/count
```

不适用维度：

```text
pagination
sort
empty
invalid_filter
```

## 执行

通过 `POST /coverage/waivers` 创建 20 条显式 waiver：

```text
reason = singleton/aggregate endpoint: pagination/sort/empty/invalid_filter not applicable to this API
owner = 42
expires_at = 2026-12-31T00:00:00Z
evidence = {"rule":"singleton_or_aggregate"}
```

## 结果

```text
total_required 441 -> 421
mapped 105
passed 90
failed 7
blocked 8
unmapped 316
design_coverage 24.9%
```

没有把这些 Cell 改成 passed；只是从 required 分母移除，并保留 waiver 审计记录。

## 追加：messages/trend

`GET /api/v1/admin/messages/trend` 是聚合时间序列，不是分页集合。  
又对 4 个维度创建 waiver：

```text
pagination / sort / empty / invalid_filter
```

结果：

```text
total_required 421 -> 417
design_coverage 25.2%
```
