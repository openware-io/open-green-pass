# 贡献指南

感谢你对 GreenPass 的关注。本项目为**纯前端原型工程**（React + Vite + TypeScript），用于产品原型沟通与设计打磨。原型阶段**不做后端业务**，一切改动在原型内完成。

## 开发流程

1. 安装依赖：`npm install`
2. 启动开发服务：`npm run dev`（localhost:5173，URL 形如 `#/target`）
3. 每次改动同步更新产品需求文档 [`docs/PRD.md`](./docs/PRD.md) 的变更记录表（追加一行 vX.Y.Z）。
4. 完成校验：
   - `npm run typecheck`
   - `npm run lint:eslint`（历史告警须清零）
   - `npm run build`
5. 生成可独立打开的单文件原型：将 `dist/output` 的 JS/CSS 内联为 `dist/GreenPass-standalone.html`（HashRouter，直接双击打开）。

## 提交规范

- Commit message 建议：`<type>: <summary>`
  - `feat:` 原型功能 / 交互
  - `docs:` 需求文档 / 说明
  - `chore:` 工程配置 / 开源规范
  - `fix:` 缺陷
- 涉及需求变更时，在 commit message 标注 `PRD vX.Y.Z`。

## 分支与 PR

- 主干为 `main`，直接推送（原型单人迭代）；多人协作时请使用分支 + Pull Request。
- 提交 PR 前请确保 CI（typecheck / lint / build）通过。

## 行为准则

请遵循 [CODE_OF_CONDUCT](./CODE_OF_CONDUCT.md)。