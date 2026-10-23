# GreenPass kind 发布记录：0.1.148-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.148-SNAPSHOT` |
| 部署 checkout | `2072667` |
| schema | 48 clean |
| server / worker | `sha256:63f74b98925603872cce67a722513e943d168fab2e5512df3a5ad2706a53e81b` |
| web | `sha256:633731d2e58854e87ecfe154c1ef36009ab8acf788ef0e7a6001168ebf051817` |
| migrate | `sha256:3ad37386f8448f6c1e180c2fb111c30f1a4093efe1dca7e9f0386cf945701a19` |
| dsh-gateway | `sha256:cc6b64de64451927c0e47fe09444fa25ff08059ef77cd703825c5f0da89cfc6f` |

## 变更

- DSH usage 增加 `cacheReadTokens` 采集与回传。
- `cost_line_item.cache_read_tokens` 写入真实缓存读 token。
- 缓存读按独立单价参与金额计算。
- 生成成本继续按实际模型 `dsh/<model>` 解析价目。

## 验证

- 实际模型：`dsh/deepseek-flash`
- 价目：输入 0.002 / 输出 0.008 / 缓存 0 元/千 token
- 成本行与 run 聚合已通过
- Go 测试、前端 typecheck 通过
- schema 48 clean
