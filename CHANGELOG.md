# Changelog

本项目所有重要变更都会记录在 CHANGELOG 中（格式参照 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)）。

## [Unreleased]
- **M2 fixtures/环境租约（0.1.112-SNAPSHOT）**：新增 run fixture prepare/list/cleanup 与 env lease/release API；验证 prepare/list/cleanup/lease/release；evidence 上传仍待补。
- **M2 归因闭环（0.1.110-SNAPSHOT）**：新增 `POST /runs/{id}/triage`，把 failed/blocked run_case_result 转为 finding，回写 `failure_class`/`finding_id`；execution 经 FindingPort 调 governance，finding 带仓库/提交/证据摘要。fixtures/evidence 上传/真实执行仍待补。
- **M1 完成：真实 DSH 生成 12 条 user 用例（0.1.106-SNAPSHOT）**：迁移 43 修复审计列；新增 `POST /generation/jobs/{id}/cases:import`，解析 DSH JSON、创建 cas_case/version、回填 cov_cell.case_id；user 模块 24/24 Cell 覆盖、uncovered=0；真实 SDK runner 本地验证，集群 gateway 仍为 stub。
- **M1 gateway 部署（0.1.103-SNAPSHOT）**：新增 `gp-dsh-gateway` Deployment/Service/SA、`gp-dsh-auth` token、server 注入 `GP_DSH_GATEWAY_URL`、迁移 42 修复 `gen_job.updated_by`；生成 Job 端到端成功（stub runner），真实 SDK runner 仍待镜像/启用。
- **M1 生成 Job 控制面（0.1.100-SNAPSHOT）**：新增 `gen_job/gen_job_event`、`GenerationJobService`、`POST/GET/DELETE /ai/generation-jobs`、`DSHGatewayClient`；当前 gateway URL 未配置，提交返回 400，真实 SDK runner 待接入。
- **M1 覆盖账本（0.1.99-SNAPSHOT）**：新增 `ctr_endpoint/ctr_parameter/ctr_error`、`cov_cell.module`；覆盖领域 archetype→Cell 枚举/稳定 hash/汇总；API `POST /coverage/plan`、`GET /coverage/cells`、`GET /coverage/summary`；user 模块 4 端点生成 24 个必需 Cell。DSH gateway/生成用例仍未完成。
- **M0-Dev 完成：Finding 最小闭环（0.1.98-SNAPSHOT）**：迁移 34-38（def/cov/evd/fixture + cas_case/run_case_result 加列）；Finding 领域状态机/去重/仓储/API（列表/详情/创建/流转）；缺陷中心接真实 API；创建/去重/流转/审计验证通过；无截图时明确提示，不再伪造证据。
- **Schema 变更策略放宽（2026-10-22）**：开发/本地允许直接 DDL、drop/recreate、启动建表；现有表可直接 ALTER/DROP/类型变更；无历史数据回填/双写/兼容视图。测试/CI 合入前固化为迁移；生产仍走版本化迁移 + 快照/回滚/审批；新增 `SCHEMA-CHANGE-CHECKLIST` 做范围控制。
- **执行/Fixtures/垂直切片规格（2026-10-22）**：新增 `EXECUTION-FIXTURE-SPEC`（环境模型、run_fixture/env_lease、状态机、Runner、失败分类、证据采集）与 `GP-VERTICAL-SLICE-1`（user 模块 4 端点/22 Cell/12-16 用例/5-7 天闭环）；新增 SEC-EX-001 模型密钥环境变量注入例外（待安全签署）。
- **工程规范合规审查与修正（2026-10-22）**：审查 AI 测试引擎方案与原型，发现表前缀、领域归属/RLS/可信写、迁移编号、OpenAPI、测试分层、gateway 镜像/部署入口、沙箱 egress 等缺口；已登记 `def_`/`cov_`、`cmd/dsh-gateway`、gateway 镜像与 ADR-001，补充迁移编号/RLS/可信写/OpenAPI/测试/沙箱约束；原型 sky/violet 已按 DESIGN-SPEC 改为 emerald/amber/red/slate，并发布 `0.1.95-SNAPSHOT`。
- **AI 测试引擎实施规格（2026-10-22）**：新增 `DSH-SDK-GATEWAY-SPEC`（一 job 一进程/warm pool、取消/超时/崩溃恢复、队列/事件/错误码）、`COVERAGE-MODEL-SPEC`（端点 archetype、Cell 模型、pairwise、覆盖门禁与 DDL）、`FINDING-EVIDENCE-SPEC`（缺陷状态机、仓库/提交、截图/视频证据、上传/保留/归因规则）。
- **缺陷仓库与多媒体证据（0.1.94-SNAPSHOT）**：缺陷列表/详情增加归属仓库、分支与冻结提交；新增“多媒体证据”区块，截图必需、视频建议，支持缩略图/大图/播放占位、证据策略与 SHA-256/MIME/时长说明；同步更新 AI 测试引擎方案的 finding 仓库字段与 evidence API。
- **缺陷优先原型（0.1.93-SNAPSHOT）**：新增缺陷中心（列表/详情/期望实际/证据链/根因/只读建议/流转）、测试执行“缺陷候选与归因”、生成页“覆盖计划”、门禁“缺陷与覆盖门禁”、报告“缺陷与建议摘要”；明确源码只读、不写回仓库；当前为前端原型数据。
- **AI 测试引擎可执行方案（2026-10-22）**：新增 `docs/plans/GP-AI-TEST-ENGINE-EXECUTION-PLAN.md`，明确 DSH 官方 headless/sdk/acp 能力边界、SDK 长驻网关与自建进程池/队列方案、覆盖账本/缺陷模型/失败归因/只读护栏、M0-M4 交付与验收指标。
- **DSH 用例生成对照评估（2026-10-22）**：对 `open-chat-admin` 2.2.1 做单轮 API 与 DSH headless 对照；基线 5 条/5 模块/仅 GET/1 条 status 断言，DSH 40 条/18 模块/43 个端点（49.4%）/170 条断言；结论：DSH 显著提升清单质量，但源码解析单次全量仍不满足大型项目“可靠、专业、覆盖全面”，需分层生成 + 覆盖率账本 + 执行反馈。详见 `docs/status/DSH-GENERATION-EVALUATION-2026-10-22.md`。
- **项目默认模型与发现反馈（0.1.91-SNAPSHOT）**：模型页增加项目/模型下拉，可直接切换项目 `model-binding`；官方模型获取增加成功 toast 和“已获取 N 个模型”状态；模型卡片改为可点击选中态，已启用模型用绿色“已启用”标识。
- **模型页信息架构重做（0.1.90-SNAPSHOT）**：删除 4 张大供应商卡、重复的“已配置供应商”和“已配置模型”两段；收敛为一张供应商表（供应商/状态/Base URL/模型/操作），一行一个供应商；未配置供应商用描边“配置”，已连接用“管理模型”。
- **官方模型目录（0.1.88-SNAPSHOT）**：移除代码中的硬编码模型版本列表，只保留供应商默认配置；新增 `POST /model-provider-models` 从供应商官方 `GET /models` 动态发现模型；保存连接时校验模型 ID 必须在官方列表中；模型页不再展示 GPT-6/Claude Opus/Qwen3.8 等内置假版本。
- **生成页项目级模型选择（0.1.87-SNAPSHOT）**：生成页模型横幅增加“生成模型”下拉，列出团队已审批模型；切换即更新项目 `model-binding`，支持在 DeepSeek V4 Pro / Flash 间切换。
- **供应商级模型选择（0.1.86-SNAPSHOT）**：模型连接按供应商聚合，一个供应商配置一次 Token；新增模型通过“启用”复用既有 Secret，无需重复输入 Token；修正 DeepSeek 预置模型为 `deepseek-flash`；后端 `POST /model-connections` 支持空 Token 读取既有 Secret 探测。
- **AI 模型连接（2026-10-22）**：接入 DeepSeek `deepseek-v4-pro`（API / OpenAI-compatible，Secret 托管），im-saas 项目绑定切换为 DeepSeek；下线原 GPT `gpt-5.6-terra` 连接并删除其 Secret。实测生成批次 `2902560158584832` 完成 5 条候选（均通过质量校验，进入 review）。
- **生成用例代码引用确认（0.1.85-SNAPSHOT）**：确认面板逐服务显示并可直接修改“本次代码引用”（分支/Tag + SHA）；打开确认前为所有已选服务加载引用；创建时不再静默回退默认分支，缺少引用明确阻断；区分“当前查看服务的代码来源”和批量确认范围。
- **生成页操作文案（0.1.84-SNAPSHOT）**：主操作由“批量生成用例”改为“生成用例”，仅在选中服务时显示数量后缀（如“生成用例（1）”），避免单选场景的批量语义歧义；同步调整“确认创建”“为所选服务创建生成批次”等辅助文案。
- **SCM 源码连接加固（0.1.83-SNAPSHOT）**：PAT 创建显式写入 `auth_type`；错误映射、失败 Secret 清理、删除/轮换 API 与 UI、trusted audit、classic scope 拒绝、绑定前仓库 Contents 探测、GitHub App `installation_account_login`/FrontendURL 校验、非数值 actor 稳定哈希、分支/Tag 分页与 GitHub 限流退避；新增后端单元测试和 SCM 弹窗组件；修正 Web 静态资源缓存策略，避免发布后浏览器继续使用旧 shell。
- **AI 助手架构评审稿**：新增 [`docs/architecture/AI-ASSISTANT-DESIGN.md`](./docs/architecture/AI-ASSISTANT-DESIGN.md)，定义显式助手与隐式助手的统一运行时、上下文/技能/工具/审批边界、业务接入矩阵、AI 对 GP 的语义命令与受限界面代理、全量操作审计、事件与 Temporal 接入、数据模型、安全审计和分阶段落地路径。
- **AI 协作原型与实施稿**：新增“AI 协作”原型页、全局/门禁页引导入口，以及 [`docs/plans/AI-ASSISTANT-IMPLEMENTATION-PLAN.md`](./docs/plans/AI-ASSISTANT-IMPLEMENTATION-PLAN.md)。原型明确呈现后台建议不操作界面、显式引导定位/预填但由用户提交、每一步进入审计链的交互边界。

## [v1.8.2] - 2026-09-29
- **执行方案全阶段可执行（v0.3）**：P1 细化到实现级（GP1-01~08 含实施步骤/关键文件/验证，im saas 首个 SUT 闭环）；P2–P4 从执行纲要细化为任务级施工图（GP2-01~08 异构执行+调度+扩展 / GP3-01~07 AI 治理+W1 / GP4-01~06 平台化）；确认点调整（P2–P4 按图执行，阶段 DoD→用户验收）。## [v1.8.1] - 2026-09-29
- **实施执行方案**：新增 [`docs/plans/IMPLEMENTATION-PLAN.md`](./docs/plans/IMPLEMENTATION-PLAN.md) v0.2（实现级）——基于工程规范细化到可执行步骤。

## [v1.8.0] - 2026-09-29
- **技术方案定稿（可执行）**：新增 [`docs/architecture/TECH-DESIGN.md`](./docs/architecture/TECH-DESIGN.md) v1.0 可执行稿。
- **后端全量工程规范**：新增 [`docs/engineering/ENGINEERING-SPEC.md`](./docs/engineering/ENGINEERING-SPEC.md) v1.0。
- **文档归属调整**：后端 Go 工程与技术方案文档统一归入后端服务仓库 `./docs/`；`open-green-pass` 仅保留原型 + 产品（PRD）/ 设计（DESIGN-SPEC）文档。

## [v1.7.6] - 2026-09-27
- **术语规范**：全平台「资产 / 资产树 / 资产库」统一为测试领域规范术语「被测对象 / 被测对象树 / 被测对象库」（SUT）。
- **被测对象层级定义**：被测工程 → 被测服务组 / 被测服务 → 被测模块 → 被测应用 / 被测端。
- **测试执行页定位**：由「回跳测试中心」改为独立完整的「执行任务总览清单」视角。

## [v1.7.5] - 2026-09-27
- 资产树 ↔ 被测对象画像关系链完善：层级标签补全（工程 / 模块）、画像页新增「测试场景覆盖」卡（资产 → 测试中心下钻）、全局 / 局部粒度标注。

## [v1.7.4] - 2026-09-27
- 全局 ↔ 测试中心下钻联动：门禁场景规则总览、执行发起入口、用例库所属场景、报告场景分块、资源场景映射，均可下钻测试中心对应场景闭环。

## [v1.7.3] - 2026-09-27
- 场景执行启动 + 用例范围选择：默认全部执行，可按用例树范围人工勾选当次用例。

## [v1.7.2] - 2026-09-27
- 契约测试统一收进测试中心 12 场景闭环；「测试场景中心」改名「测试中心」。

## [v1.7.1] - 2026-09-27
- 测试中心聚焦交互：点击卡片放大、其他隐藏、下方内容联动、可回退；卡片任务进行中提示。

## [v1.7.0] - 2026-09-27
- 测试中心场景 × 闭环矩阵重构：12 个测试场景 ×（用例 / 执行 / 门禁 / 历史 / 报告）五闭环；报告支持跨场景汇总合编；菜单结构重组、图标全唯一；测试资源页聚焦资源池 + 场景 × 资源映射。
