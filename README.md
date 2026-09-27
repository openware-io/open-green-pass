# GreenPass · 软件工程测试治理平台（前端原型）

[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![CI](https://github.com/openware-io/open-green-pass/actions/workflows/ci.yml/badge.svg)](https://github.com/openware-io/open-green-pass/actions)
[![React](https://img.shields.io/badge/React-19-blue)](https://react.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-blue)](https://www.typescriptlang.org/)
[![Vite](https://img.shields.io/badge/Vite-8-purple)](https://vitejs.dev/)
> **GreenPass**：软件工程测试治理平台 —— 站在软件工程"测试阶段"的位置，对（AI 生成 / 人写的）被测工程与系统做**质量量化、测试管控与可信治理**，系统本身由 AI 驱动。

本仓库为**纯前端原型工程**（React + Vite + TypeScript + Tailwind + shadcn/ui），全部数据为本地 mock，可独立构建与运行，用于产品原型沟通与设计打磨。

## 产品定位

- **通用测试定位**：AI 生成的系统能测，人开发的软件系统同样能测 —— 不局限于"只测 AI 产物"。
- **AI 驱动实现**：平台的用例生成、质量判定、契约分析、篡改检测等能力由 AI 模型提供，**每个工程可独立选择所用的 AI 模型**。
- **面向中大型技术团队**：支持多租户团队切分、团队内人员管理与角色权限矩阵、团队级数据隔离。

## 技术栈

- 前端：React 19 + TypeScript
- 构建：Vite
- 样式：Tailwind CSS v4
- UI 组件：shadcn/ui（`@/components/ui/`）
- 图标：lucide-react
- 路由：react-router-dom（**HashRouter**，兼容 `file://` 静态打开）
- 提示：sonner（`<Toaster/>`）

## 功能页面

| 分组 | 页面 | 说明 |
|---|---|---|
| 测试闭环 | 被测对象画像 `/target` | 资产级画像（随侧边栏所选资产联动）、AI 生成过程还原、评分构成、未决风险 |
| | 上游源与生成 `/generation` | 六类上游源 → 适配器 → 用例种子 → 质量验证 → 入库 |
| | 测试用例库 `/cases` | 用例版本化、断言强度、变异分数、上游绑定 |
| | 测试执行 `/exec` | 实时进度、证据捕获、不放水不跳过 |
| | 契约测试 `/contracts` | 契约注册、变更影响、消费者验证 |
| | 质量门禁 `/gate` | 策略即代码、AI 判定、篡改检测、契约门禁（可重新判定，随所选资产联动） |
| 可信与审计 | 审计日志 `/audit` | 哈希链、仅追加、可独立验证 |
| | 需求追溯 `/trace` | 跨服务追溯矩阵 |
| | 并发与资源 `/concurrency` | 调度、隔离、资源池、冲突 |
| 组织与模型 | 团队与权限 `/teams` | 多租户团队切分、成员管理、角色权限矩阵 |
| | AI 模型配置 `/models` | 模型池、每工程模型绑定 |

## 目录结构

```
src/
├── index.tsx           # 入口（HashRouter）
├── app.tsx             # 路由配置 + <Toaster/>
├── index.css           # 全局样式 + 主题变量
├── components/
│   ├── Layout.tsx      # 全局布局（侧边栏 + 顶栏 + <Outlet context=selectedAsset>）
│   ├── shared.tsx      # KpiCard / PageHeader / PrimaryButton / GhostButton / Card
│   └── ui/             # shadcn/ui 内置组件
├── pages/              # 页面模块（每页一个目录）
│   ├── TargetPage/  GatePage/  GenerationPage/  CasePage/  ...
│   └── ...
└── data/
    └── mock.ts         # 单一数据源（含 assetToProfile 资产联动画像派生）
docs/
└── PRD.md              # 产品需求文档（随原型迭代同步更新）
dist/
└── GreenPass-standalone.html   # 可独立打开的单文件原型（内联 CSS/JS）
```

## 运行方式

```bash
npm install
npm run dev        # 开发服务（localhost:5173，URL 形如 #/target）
```

**构建 / 生成可独立打开的单文件原型：**

```bash
npm run build      # 产物在 dist/output + dist/output_resource
```

仓库内已内置 `dist/GreenPass-standalone.html` —— 将 JS/CSS 内联、HashRouter 路由，**直接双击即可在浏览器打开**（无需启动服务），适合快速预览与分享。

## 质量校验

```bash
npm run typecheck      # tsc -p tsconfig.app.json
npm run lint:eslint    # eslint src
```

## 说明

- 原型阶段**不做后端业务**，所有交互（重新判定、搜索、成员/模型配置、资产联动）均为前端 mock 反馈。
- 产品需求文档见 [`docs/PRD.md`](./docs/PRD.md)。
- 开源许可：[MIT](./LICENSE)。

---

## 参与贡献

- 贡献指南：[CONTRIBUTING.md](./CONTRIBUTING.md)
- 行为准则：[CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)
- 变更记录：[CHANGELOG.md](./CHANGELOG.md)
- 安全政策：[SECURITY.md](./SECURITY.md)
- Issue / PR 模板：[.github](./.github)

---

*GreenPass 原型 · 持续迭代中。*
