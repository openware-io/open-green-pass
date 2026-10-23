# DSH 真实生成链路（2026-10-23）

## 变更

### Gateway 真实 SDK

- `Dockerfile.dsh-gateway` 改为 `node:22-bookworm-slim`
- 镜像内安装 `@deepseek-ai/dsh@0.2.1-rc.2`
- `GP_DSH_RUNNER=sdk`、`GP_DSH_PROVIDER=deepseek-official`、`GP_DSH_MODEL=deepseek-flash`
- API key 从 `gp-model-1001-deepseek-api/token` 注入
- `/tmp` emptyDir，UID 1000，只读根文件系统

### 生成批处理改为 DSH

- worker 增加 `GP_DSH_GATEWAY_URL` / `GP_DSH_INTERNAL_TOKEN`
- `GP_GENERATION_ENGINE=dsh`
- `GenerationBatchStore.SetDSHClient`
- `GenerateModelSeed` 在 DSH 模式下：
  - 提交内部 DSH job
  - 轮询到终态
  - 解析 `cases` 输出
  - 生成 seed 的 provider/model 记为 `dsh / dsh-sdk`
- 不再要求项目级 `model_binding`

### 生成页

- 移除“模型生成 / 当前绑定 deepseek-flash / 已配置模型 2 个”
- 改为“DSH 生成”，说明模型由 DSH Profile 配置
- 不再加载/切换项目模型绑定

## 验证

### DSH smoke

`POST /api/ai/generation-jobs`：

- job `2905358401474560`
- state `succeeded`
- output `{"cases":[]}`

### 真实 pc-admin 生成批次

- batch `2905359519387648`
- state 从 `fetching` → `generating` → `review`
- 生成 7 条 seed
- seed provider `dsh`，model `dsh-sdk`
- source locator 示例：
  `model://dsh/dsh-sdk?...&source=src/api/config.js`

## 当前版本

`0.1.125-SNAPSHOT`

## 说明

- 真实生成不依赖项目模型绑定；模型由 gateway 的 DSH Profile 决定。
- `GP_GENERATION_ENGINE` 非 `dsh` 时仍保留旧模型路径，作为非 DSH 部署回退；当前 kind 明确为 `dsh`。
