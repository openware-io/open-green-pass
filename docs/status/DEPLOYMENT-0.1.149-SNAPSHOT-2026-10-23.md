# GreenPass kind 发布记录：0.1.149-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.149-SNAPSHOT` |
| 部署 checkout | `3e0b919` |
| schema | 48 clean |
| server / worker | `sha256:d51635da3a9c0de11930b6f6510b129f71a8dacd77497f2c35ca342192901132` |
| web | `sha256:633731d2e58854e87ecfe154c1ef36009ab8acf788ef0e7a6001168ebf051817` |
| migrate | `sha256:149a4ae65e4d54f4b0c48adcf439492834eff2b6931039e6b7a3a32d99446b0a` |
| dsh-gateway | `sha256:ee849584741139ce4fb51e10aba351731b3548b2bf566b35295a9ccc63b3cf60` |

## 变更

- 修复 000048 迁移中的字面量换行，使价目迁移可重复执行。
- 价目查询只匹配 `status=active` 且未过期的价格。
- 增加拆分计价单元测试。

## 验证

| 检查 | 结果 |
|---|---|
| schema | 48 clean |
| 价目迁移重跑 | 通过 |
| 价目创建/查询 | 通过 |
| 拆分成本 | 234/3137 → ¥0.025564 |
| 实际模型 | `dsh/deepseek-flash` |
| Go 测试 | 通过 |
| 前端 typecheck | 通过 |

## 仲裁代理

新增：

- `scripts/dev/arbiter-generation.ps1`
- `docs/status/ARBITER-COMPLETION-2026-10-23.md`

最近一次只读仲裁：26 项检查，0 失败。
