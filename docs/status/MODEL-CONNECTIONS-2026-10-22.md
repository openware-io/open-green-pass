# AI 模型连接变更记录（2026-10-22）

## 变更范围

| 项目 | 值 |
|---|---|
| 团队 | `1001` |
| 项目 | `im-saas`（`2876690473259008`） |
| 运行版本 | `0.1.85-SNAPSHOT`（本次仅运行时配置，无镜像变更） |
| 操作人 | `zhangli` |

## DeepSeek 接入

| 项目 | 值 |
|---|---|
| 连接 ID | `2902559098867712` |
| Provider | `deepseek` |
| Model Key | `deepseek-v4-pro` |
| Protocol | `api` / OpenAI-compatible |
| Base URL | `https://api.deepseek.com/v1` |
| 状态 | `approved` |
| 凭据状态 | `configured`（实际生成已成功） |
| Secret | `gp-model-1001-deepseek-api`（仅存 token，不落库、不回显） |
| 项目绑定 | `model_binding = {provider: deepseek, model: deepseek-v4-pro}` |

接入方式：通过平台 `POST /model-connections` 写入连接，服务端探测 `GET /v1/models` 成功；随后 `PUT /targets/2876690473259008/model-binding` 绑定到 im-saas。

## GPT 下线

| 项目 | 值 |
|---|---|
| 原连接 ID | `2880382758227968` |
| 原 Provider / Model | `openai` / `gpt-5.6-terra` |
| 处理 | 删除 `mdl_model` 记录；删除 K8s Secret `gp-model-1001-openai-api`；解除项目绑定并切换到 DeepSeek |
| 影响 | 模型目录和“已配置连接”中不再出现 GPT；生成页只显示 DeepSeek |

OpenAI 的预置模型仍保留在平台目录中（显示“未配置”），仅作为可选厂商目录，不代表已接入。

## 生成验证

| 项目 | 值 |
|---|---|
| 批次 ID | `2902560158584832` |
| 代码引用 | `tag / 2.2.1` |
| 冻结 SHA | `a44876435f69f78723e23c04d2ff1d59e1d05f96` |
| 候选用例 | 5 条，`quality_state=passed`，`approval_state=pending` |
| 批次状态 | `review`（等待人工审核） |
| 模型来源 | `model://deepseek/deepseek-v4-pro` |

该批次已实际调用 DeepSeek 完成生成，说明连接、凭据、项目绑定和生成链路均生效。

## 安全说明

- DeepSeek token 仅保存在 K8s Secret，数据库只保存 `k8s://gp/gp-model-1001-deepseek-api#token` 引用。
- 该 token 已在对话中暴露，建议尽快在 DeepSeek 控制台轮换；轮换后重新调用 `POST /model-connections`（同 provider/model/base_url，新 token）即可更新 Secret 与连接，无需重启服务。
- 不要将 token 写入文档、日志或代码。

## 验收清单

- [x] `GET /model-catalog` 仅返回 DeepSeek 已配置模型
- [x] `GET /model-connections` 仅返回 DeepSeek 连接
- [x] `im-saas` 模型绑定为 `deepseek / deepseek-v4-pro`
- [x] 生成页显示 `deepseek / deepseek-v4-pro`，不再显示 GPT
- [x] DeepSeek 实际生成 5 条通过质量校验的候选，批次进入 review
