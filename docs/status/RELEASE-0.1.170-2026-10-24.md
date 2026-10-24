# GreenPass 0.1.170-SNAPSHOT 功能与 M3 权限覆盖（2026-10-24）

## 新增

- case header 支持受控环境引用：

```text
Authorization: env:GP_CASE_NONADMIN_TOKEN
```

仅允许 `GP_CASE_` 前缀。

- Secret `gp-case-auth` 注入 server/worker。
- 普通用户 token 只存在 Secret，不写入用例。

## M3

- 61 个 authz Cell 新增非管理员拒绝用例
- authz mapped/pass = 65/65
- total mapped = 166
- passed = 151
- failed = 7
- blocked = 8
- design_coverage = 39.8%

run `2908158313598976`：61 cases，122 results（attempt 1/2），全部 pass。

## 新 finding

```text
2908160974196736  P1
手动 execute 与调度器重复执行同一 run
```

## 版本

```text
0.1.170-SNAPSHOT
schema 48 clean
```

## 未完成

```text
M3 251 Cell 缺 fixture
M4 egress 在 kind CNI 下未生效
P0/P1 未关闭
0.1.170 镜像尚未推送/签名
```
