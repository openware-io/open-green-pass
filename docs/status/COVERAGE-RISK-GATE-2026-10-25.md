# 覆盖门禁风险模式（0.1.174-SNAPSHOT）

## 背景

原门禁只支持严格模式：

```text
design 100%
execution >= 90%
P0/P1 = 0
```

对当前测试环境，100% 需要大量缺失 fixture，容易诱导伪造。新增风险验收口径，但不改变 Cell 事实。

## API

```text
GET /gates/coverage?project_id=...&mode=strict
GET /gates/coverage?project_id=...&mode=risk
```

默认 `strict`，保持兼容。

## risk 规则

```text
1. 每个模块至少 1 个 passed Cell（18 模块 smoke）
2. 高风险模块 design >= 90%
3. 高风险模块 execution >= 90%
4. open P0 = 0
5. open P1 = 0
```

高风险模块：

```text
auth
user
security
config
openPlatform
clientRelease
execution
```

## 当前结果

```text
mode=risk
total_required=417
design_covered=277
passed=223
gate_passed=false
open_p0=1
open_p1=3
```

主要阻断：

```text
auth design/execution 10.0% < 90%
clientRelease 64.4% < 90%
config 56.3% < 90%
openPlatform 78.6% < 90%
security 71.4% < 90%
P0/P1 = 1/3
```

## 修复

同时修复门禁 JSON 字段冲突：

```text
passed -> 通过 Cell 数量
gate_passed -> 门禁布尔结果
```

与 OpenAPI 的 `gate_passed` 保持一致。
