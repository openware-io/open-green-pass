# GreenPass kind 发布记录：0.1.150-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.150-SNAPSHOT` |
| 部署 checkout | `17197b0` |
| schema | 48 clean |
| server / worker | `sha256:d51635da3a9c0de11930b6f6510b129f71a8dacd77497f2c35ca342192901132` |
| web | `sha256:633731d2e58854e87ecfe154c1ef36009ab8acf788ef0e7a6001168ebf051817` |
| migrate | `sha256:149a4ae65e4d54f4b0c48adcf439492834eff2b6931039e6b7a3a32d99446b0a` |
| dsh-gateway | `sha256:ee849584741139ce4fb51e10aba351731b3548b2bf566b35295a9ccc63b3cf60` |

## 收尾

- `ListByProject` 改为 run 列表一次查询 + unit 批量查询，移除 N+1。
- 重复价目返回 `409 cost_price_exists`。
- 新增 `GenerationRunService` 创建/取消级联单测。
- 仲裁代理复跑通过。

## 仲裁

```text
Checks: 26
Failed: 0
VERDICT: PASS
```

仲裁报告：[ARBITER-COMPLETION-2026-10-23.md](ARBITER-COMPLETION-2026-10-23.md)
