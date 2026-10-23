# GreenPass kind 发布记录：0.1.156-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.156-SNAPSHOT` |
| schema | 48 clean |
| 变更 | `GP_RUNNER_TYPE=http` 真正启用 HTTP Runner；兼容旧扁平用例 |
| 验证 | 12 条旧用例实际发出 HTTP 请求；2 通过 / 10 个占位符 500 |

## 修复

- worker switch 增加 `case "http"`；此前 `http` 落入 MockRunner，执行结果全是 `mock ok`。
- `hasDeclarativeHTTPRequest` 识别旧格式 `{path,method,expected_status}`，从而解析环境 BaseURL。
- HTTP Runner 接受旧格式并合成 status assertion。

## 验证

run `2906703985086464`：

```text
2 pass
10 fail（{id} 占位符导致 500）
```

这说明真实 runner 已生效，不再是 mock。

## 说明

- 旧用例的 `{id}` 需要测试数据绑定，不能直接自动执行 PUT/DELETE。
- 覆盖同步仍为 executed=24，但 10 条是占位符失败，M3 未关闭。
