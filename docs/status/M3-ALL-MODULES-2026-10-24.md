# M3：pc-admin 全模块覆盖账本（2026-10-24）

## 范围

从目标前端 `open-chat-admin/src/api/*.js` 盘点 API 调用，导入 18 个模块、71 个端点：

```text
accountCancellation  audit              auth
clientRelease        config             deviceToken
friend               group              media
message              miniappService     monitor
openPlatform         security           server
stats                user               userSticker
```

导入接口：

```text
POST /coverage/plan
```

## 账本结果

```text
modules=18
cells=441
user cases mapped=24
other module cases mapped=0
```

分模块：

```text
accountCancellation 19   audit 7       auth 10
clientRelease 45        config 20      deviceToken 13
friend 7                group 18       media 49
message 14              miniappService 59
monitor 34              openPlatform 14
security 60             server 21      stats 14
user 24                 userSticker 13
```

## 当前含义

- “18 模块覆盖计划”已落地，不再是仅 user 模块。
- 除 user 外，441-24=417 个 Cell 仍是 `planned`，没有用例映射。
- 设计覆盖、执行覆盖不能标记完成；需要为 17 个模块生成/导入用例。

## 后续

- 按模块用 DSH 生成用例并映射 Cell。
- 对每个模块执行覆盖回写。
- 最后按模块门禁汇总。
