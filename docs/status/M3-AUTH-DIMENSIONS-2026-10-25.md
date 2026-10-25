# M3 auth 维度补齐（2026-10-25）

## 新增

为 auth 模块 8 个 Cell 创建真实请求：

```text
bad_credentials  login / sso
expired_token    login / sso
lockout_rate_limit login / sso
reset_invalid    login / sso
```

## 结果

run `2910689558237184`：

```text
6 pass
2 fail
```

失败：

```text
lockout_rate_limit 返回 401，不是 429
```

auth 模块：

```text
required=10
mapped=9
passed=7
design=90.0%
execution=90.0%
```

auth 达到风险门禁 90% 阈值，但不代表 lockout 行为正确；401/429 差异保留为失败。

## 当前 M3

```text
required = 417
mapped   = 309
passed   = 244
failed   = 57
blocked  = 8
planned  = 132
design_coverage = 68.3%
```

## 风险门禁

```text
mode=risk
design_covered=285
passed=229
gate_passed=false
open_p0=1
open_p1=3
```

剩余高风险阻断：

```text
clientRelease 64.4%
config 56.3%
openPlatform 78.6%
security 71.4%
```
