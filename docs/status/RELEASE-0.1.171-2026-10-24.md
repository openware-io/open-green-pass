# GreenPass 0.1.171-SNAPSHOT 发布验证（2026-10-24）

## 修复

- 已调度 run 的 `POST /runs/{id}/execute` 返回 202，不再重复执行。
- 未启用 schedule queue 时仍允许手动执行。

## 验证

run `2908188712701952`：

```text
POST /execute -> 202
state -> done
每个 case -> 1 条结果
```

重复执行 P1 finding 已关闭。

## 版本

```text
0.1.171-SNAPSHOT
schema 48 clean
arbiter 26/26 PASS
```

## 镜像

```text
open-green-pass:0.1.171-SNAPSHOT
sha256:0881fb158c1585e8687d8780109c581a910999f6030ad704344746e9fba8992a
（已签名并 verify 通过）

web/migrate/dsh-gateway 沿用上一版已签名 digest。
```

## M3

```text
required=417
mapped=301
passed=238
failed=55
blocked=8
unmapped=140
waived=24
design_coverage=66.4%
```

## 未完成

```text
M3 140 Cell 缺 fixture/合同
M4 egress 在 kind CNI 下未生效
P0/P1 未关闭
```
