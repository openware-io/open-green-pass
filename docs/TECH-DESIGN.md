# GreenPass · 技术方案与技术栈选型

> 文档定位：基于 PRD v1.8.1 的**实现层技术方案**，面向生产落地（SaaS + 私有化双形态、中大型技术团队）。
> 视角：技术评审伙伴——每个选型给出**推荐 + 备选 + 生产落地理由 + 业界对标**，明确判断而非罗列。
> 状态：**可执行稿 v1.0**（技术栈 Go+Temporal 已定稿；工程规范见 ENGINEERING-SPEC.md，UI/交互见 DESIGN-SPEC.md）

---

## 0. 决策摘要（TL;DR）

| 关注点 | 结论 |
|---|---|
| 系统本质 | 一个**AI 驱动的、多租户的、面向软件的测试治理平台**：AI 生成用例 + 异构测试执行引擎 + 可信审计 + 成本治理 |
| 后端语言 | **Go**（执行调度/沙箱/事件流重 IO）为主；业务 CRUD 模块化。备选：Java(Spring Boot) 若团队 Java 主导 |
| 主数据库 | **PostgreSQL 15+**（多租户 RLS + JSONB 灵活模型） |
| 时序数据 | **TimescaleDB**（成本趋势/执行历史），或 Prometheus + 自研回放 |
| 对象存储 | S3 兼容（MinIO 私有化）——测试证据（截图/日志/脚本） |
| 工作流编排 | **Temporal**（AI 生成管道、测试执行流水线：长时任务/重试/可观测/暂停恢复） |
| AI 网关 | **统一模型网关（LiteLLM/OneAPI/自研）+ Token 计量 + 成本归因**——"每工程选模型"与成本审计的落地核心 |
| 策略引擎 | **OPA (Rego)** 质量门禁"策略即代码" |
| 前端 | 沿用 **React 19 + Vite + TS + Tailwind4 + shadcn/ui**，与后端纯 API 解耦，报告导出在服务端 |
| 执行沙箱 | K8s（Job/Pod 隔离沙箱）+ 真机池（STF 自建）+ 浏览器集群（Playwright） |
| 资源与任务编排 | 集中式调度器 + 多级配额（租户→工程→资产）+ Temporal 承载执行状态机；**多团队并行时防止单团队耗尽资源**（详见 §4.x） |
| 可信审计 | **事件溯源 + 哈希链**（Append-only 表 + prev_hash，自实现，轻量） |
| 报告导出 | 服务端模板：HTML / PDF(Chromium) / Word(docx) / Markdown |
| 部署 | 微服务模块化 + 单体优先，Docker + Helm，私有化可单二进制 |

---

## 1. 系统本质与核心架构挑战

GreenPass 不是普通"测试管理工具"，它有四个本质特征，决定了技术选型：

1. **AI 是第一公民，且模型可配置**：用例生成、质量判定、契约分析、篡改检测全由 AI 驱动，且**每个工程绑定不同模型**。→ 必须有**统一 AI 网关**：路由、计量、成本归因、fallback、可观测。
2. **异构测试执行**：12 类场景横跨 后端(K8s沙箱) / Web(浏览器) / 移动端(真机池) / AI专项，并发执行、资源池调度、隔离与冲突。→ 重 IO、并发高，这是性能与调度主战场。
3. **可信审计是产品核心价值**：测试证据(截图/日志)哈希锚定、操作审计留痕、"测了也白测"的环境版本校验、成本可追溯。→ **审计链 + 不可变数据**是一等公民，非附赠功能。
4. **多租户 + 资产级细粒度权限**：团队即租户、数据隔离、工程负责人可向下分配。→ 租户模型 + 资产级 RBAC 贯穿全数据。

> 核心判断：**执行调度 + AI 编排 + 可信审计**是技术护城河；业务 CRUD（团队/用例/报告）是通用能力，用成熟框架快速覆盖。

---

## 2. 总体架构（分层）

```
┌────────────────────────── 接入层 ──────────────────────────┐
│  Web SPA(React)  ·  CI/CD(Webhook/CLI/API)  ·  第三方(SSO)  │
└──────────────────────────────┬─────────────────────────────┘
                               │ REST(OpenAPI) / WebSocket/SSE / 事件
┌──────────────────────────────▼─────────────────────────────┐
│                        API 网关 / BFF                        │
│   认证(AuthN) → 鉴权(RBAC+资产级) → 租户上下文 → 限流/配额    │
└──────┬──────────────┬────────────────┬──────────┬───────────┘
       │              │                │          │
┌──────▼─────┐ ┌──────▼──────┐ ┌──────▼─────┐ ┌──▼─────────────┐
│ 治理域(CRUD)│ │  AI 编排域    │ │  执行域      │ │  可信域         │
│ 被测对象/用例│ │ 模型网关/计量 │ │ 调度器/沙箱  │ │ 审计/门禁/成本   │
│ 契约/报告   │ │ 生成管道(Temporal)│ │ 真机/浏览器  │ │ 哈希链/OPA     │
└──────┬─────┘ └──────┬──────┘ └──────┬─────┘ └──┬─────────────┘
       └──────────────┴──────┬───────┴──────────┘
┌────────────────────────────▼───────────────────────────────┐
│  数据层                                                   │
│  PostgreSQL(业务+审计链)  ·  TimescaleDB(时序)  ·  S3/MinIO(证据)  │
│  Redis(缓存/队列/限流)  ·  Temporal(工作流持久化)  ·  Kafka?(事件总线)  │
└────────────────────────────────────────────────────────────┘
```

**关键交互流**（两个长链路是系统的命脉）：
- **AI 生成管道**（PRD R-TEST-2/28~31）：关联仓库→适配解析→AI生成种子→质量验证→人工审核→入库。**长时、多步、可重试、可回退、留审计** → 用工作流引擎(Temporal)承载，天然支持"回退本次生成"。
- **测试执行闭环**（PRD R-EXEC/R-SCEN）：启动→版本校验→调度到沙箱/真机/浏览器→执行→捕获证据→过门禁→生成报告。**并发、可暂停/恢复、实时进度推送** → 调度器 + 事件流 + WebSocket/SSE。

---

## 3. 技术栈选型矩阵

### 3.1 后端运行时

| 层 | 推荐 | 备选 | 理由（生产落地） |
|---|---|---|---|
| 主后端 | **Go** | Java(Spring Boot 3)、Node(NestJS) | 执行调度/沙箱/事件流是**高并发重 IO**，Go 的 goroutine、内存占用、单二进制私有化部署、K8s 生态是业界此类平台（CI/测试执行类）主流；`go:embed` 静态资源便于私有化交付。若团队 Java 主导，Spring Boot 完全可行（业务 CRUD 更成熟），执行域可用独立 Go 服务补齐 |
| 模块化 | 单体优先 + 按域拆内部模块 | 微服务(k8s) | 中大型团队 + 私有化双形态：**先单体演进、按域(治理/AI/执行/可信)清晰分模块**，需要再拆。过早微服务是本平台常见陷阱 |
| 事件/消息 | **Redis Streams / Kafka(高吞吐时)** | — | 执行进度、审计事件流；量级中期 Redis Streams 够，万亿级审计再上 Kafka。**审计走追加日志不依赖 Kafka 做持久化真相** |

### 3.2 数据

| 数据 | 推荐 | 理由 |
|---|---|---|
| 业务主库 | **PostgreSQL 15+** | JSONB 承载灵活模型（被测对象树/用例版本/门禁规则/成本归因快照），RLS 做多租户行级隔离，事务支撑"审计链 + 业务"原子写 |
| 多租户 | **共享库 + RLS（默认）→ schema-per-tenant(大客户可选)** | 中大型团队多数场景 RLS + `tenant_id` 索引足够且运维成本低；独立大客户私有化可整库隔离。资产级 RBAC 在应用层叠加 |
| 时序 | **TimescaleDB** | 成本趋势、执行历史、通过率趋势——连续聚合 + 下采样，避免自研回放 |
| 证据存储 | **S3/MinIO** | 截图/日志/脚本按 `tenant/object/run/case` 分层，签名 URL 短时授权，哈希值与元数据入库 |
| 缓存/队列 | **Redis** | 会话/限流/配额/队列(Streams)/热数据 |
| 工作流 | **Temporal** | AI 生成管道 + 执行流水线的**长时/重试/暂停恢复/可观测/历史**，是手工队列无法替代的 |

### 3.3 AI 层（平台的差异化核心）

| 能力 | 推荐 | 说明 |
|---|---|---|
| 模型网关 | **统一网关（LiteLLM 起步，自研增强）** | 收敛 豆包/DeepSeek/GPT/Claude/私有 为一套接口；路由、超时、fallback、并发控 |
| Token 计量 | 网关层**统一计量 + requestId 贯穿** | 每个 AI 调用关联 `requestId → 用例生成/执行/判定 业务点 → 被测对象/团队`，单价表×Token=金额，落地成本归因 |
| 可观测 | OpenTelemetry + LLM 追踪(调用/Token/时延) | 成本 + 质量双视角；"AI 决策痕迹"可溯源 |
| 门禁判定 | 判定服务(调模型) + **OPA(Rego) 规则** | 阈值/策略声明式；"策略即代码"进 Git 评审 |
| 篡改检测/变异 | 独立子域服务 | 轻量自研 + 标记学习；属护城河，逐步沉淀 |

### 3.4 测试执行引擎

| 场景族 | 执行载体 | 技术 |
|---|---|---|
| 后端/集成/API/压测 | K8s **Job/Pod 沙箱** | 一次性 Pod + 资源 Limit + 网络隔离(NetworkPolicy)；压测用 k6/Locust 容器 |
| 契约测试 | 消费者/提供者 | Pact (契约注册表 + 变更影响) |
| Web E2E / 视觉回归 / Web性能 | **浏览器集群** | Playwright（截图/视觉对比内置）、Lighthouse(性能) |
| 移动 E2E / 真机兼容 | **真机池** | STF 自建真机云 / 云真机接入，Appium + WebDriverAgent |
| 弱网 | 真机/沙箱 + **tc/netem** | 网络损伤注入 |
| AI 专项审计 | 判定服务 | 篡改/断言强度/变异/幻觉校验 |
| 资源调度 | **执行调度器** | 并发池 + 配额(租户/工程负责人) + 冲突检测 + 暂停/恢复；状态机持久化(Temporal) |

### 3.5 可信域（护城河）

| 能力 | 方案 |
|---|---|
| 操作审计 | **事件溯源 + 哈希链**：单表 `audit_events(id, ts, tenant, actor, op, asset, payload, prev_hash, hash)` 仅追加；哈希=H(prev_hash∥payload)，周期性锚定快照 |
| 测试证据 | 截图/日志 → S3 + sha256 入库，证据与 Run/用例 绑定；服务级截图策略(防性能) |
| 版本校验 | 执行前拉取 目标版本 vs 环境运行版本 比对，阻断不匹配("测了也白测") |
| 报告 | 服务端模板渲染：HTML / PDF(Chromium headless) / Word(docx 模板) / Markdown；分级下钻(工程/服务/用例) |

### 3.6 前端（沿用原型，解耦升级）

- 保持 **React 19 + Vite + TS + Tailwind4 + shadcn/ui**（原型已验证，避免重写）。
- **解耦**：所有数据由 mock 切换为 `API client`（OpenAPI 生成类型），`mock.ts` 演进为 Mock Service Worker / 本地 mock server，前端开发不阻塞后端。
- 实时执行进度：**WebSocket / SSE**。
- 重图表(echarts/recharts) + 复杂表格(shadcn/表格虚拟化)按需加载。

---

## 4. 关键架构决策（带判断）

### D1 语言：Go 为主
- **理由**：执行调度/沙箱并发/事件流是系统性能主战场，Go 的并发模型与 K8s 原生契合；单二进制 + `go:embed` 让**私有化部署成本极低**（中大型团队私有化是刚需）；内存占用低，沙箱/真机 farm 场景省资源。
- **权衡**：业务 CRUD 与报告生成的"样板代码"Go 比 Spring 略多；若团队以 Java 为主，**保持架构不变、用 Spring Boot 实现治理域 + 独立 Go 执行域** 是稳妥折中。**架构决策优先于语言偏好。**

### D2 多租户：RLS 共享库 + 资产级 RBAC
- `teams` 即租户；全表 `team_id` + PostgreSQL **Row-Level Security** 强制隔离（防漏查）。
- 资产级权限：`member × asset × (permission + resource quota)`，应用层鉴权矩阵（PRD 4.3）。用 RBAC(角色) + ABAC(资产上下文) 混合。
- 大客户私有化可整库/schema 隔离。**先 RLS，不要过早上每租户独立库。**

### D3 工作流用 Temporal，不用自研队列
- AI 生成管道（6 步 + 人工审核 + 回退）与执行流水线（版本校验→调度→证据→门禁→报告）都是**长时、多阶段、可重试、可暂停恢复**的工作流。
- 自研状态机 + 队列在三五年后会为"重试/幂等/暂停恢复/审计重建"付学费；Temporal 是业界标准（GitHub Actions 同源思想），直接把工作流历史当审计素材。

### D4 统一 AI 网关 = 成本审计的前提
- 没有网关，成本审计(R-TEST-10)与"每工程选模型"都无法落地：必须**单点计量** token 入/出 + 单价，并把 `requestId` 贯穿到业务点（用例生成/执行/判定），才能做到"总览=明细之和、存量执行成本持平或递减可证明"。

### D5 门禁"策略即代码"用 OPA
- 门禁规则（覆盖率阈值/断言强度/契约门禁/变异分数/追溯/回放）声明为 Rego，随 Git 版本化、可评审、可回滚；判定结果进审计链。比硬编码在业务代码里专业且可审计。

### D6 审计 = 事件溯源 + 哈希链（自实现，轻量）
- 无需区块链/专用审计数据库（过度设计）。**单 PostgreSQL 表 + prev_hash 哈希链 + 周期快照锚点**，即可实现"仅追加、防篡改、可独立验证"（PRD R-TEST-7）。核心是**禁止 UPDATE/DELETE + 应用层强制 append-only**。

### D7 报告服务端生成
- 前端导出 PDF/Word 依赖客户端库易碎且无法分级合编；**服务端模板渲染**：HTML/Markdown 直接模板，PDF 用 Chromium headless 打印，Word 用 docx 模板填充。跨场景汇总合编在服务端一次成型。

### D8 执行证据与性能
- 截图/日志默认进 S3 + 哈希；**服务级截图开关**（防高并发/大流量性能开销，PRD R-TEST-13）在调度器按对象策略下发，执行器遵守。

---

## 5. 数据模型核心实体（概念级）

```
Team(租户) 1─* Project/被测对象树(4层 L0-L4, 可多工程)
  ├─ 每工程绑定 AI 模型 + 绑定源码仓库/分支/版本(上游源)
  ├─ AssetRBAC: member × asset × (perm + quota)
  └─ 1─* Service 1─* Module
Case(版本化: version/versions/change新增更新删除/回退) 1─* CaseExec(执行载体: 脚本/HTTP/规则/压测 + 参数/数据/断言)
Run(测试运行, 时间维主键, 绑定被测对象当时版本/分支/环境) 1─* CaseResult
  ├─ Evidence(截图/日志, S3+sha256)
  └─ 1─* ServiceReport → 跨场景 Report(合编) → 导出HTML/PDF/Word/MD
GateRule(OPA Rego, 按被测对象派生) / GateResult
CostLineItem(requestId, 业务点: 生成/执行/判定, 模型, tokenIn/Out, 金额, 归属团队/对象/用例/Run) → 总览=Σ明细
AuditEvent(append-only + prev_hash 哈希链)
CIWebhook / TriggerRule / Connector(门禁回写)
```

---

## 6. 演进路线（原型 → 生产）

| 阶段 | 目标 | 关键动作 |
|---|---|---|
| P0 现原型 | 交互/需求定稿 | 前端 mock + PRD/DESIGN-SPEC 收敛 |
| P1 最小闭环 | 单工程跑通 | Go/后端骨架 + PG 多租户 + 用例版本化 + 一个场景执行(如 API) + 门禁判定 + 报告导出 |
| P2 异构执行 | 多场景 | K8s 沙箱 + 浏览器集群 + 真机池 + 调度器(Temporal) + 证据捕获 |
| P3 AI 治理 | 平台化 | 统一 AI 网关 + 成本归因 + 生成管道(Temporal) + 篡改/变异 |
| P4 平台化 | 中大型团队 | 资产级 RBAC 精细化 + CI/CD 对接成熟化 + 私有化交付 + 审计链锚定 |

**每阶段验收都以"数据可溯源、口径自洽"为硬标准**（原型已确立的 QA 原则延续到生产）。

---

## 7. 风险与权衡清单

| 风险 | 应对 |
|---|---|
| 异构执行引擎复杂度最高 | 分场景逐步接入，先 API/K8s 沙箱跑通闭环再横向扩展；调度器与执行器解耦 |
| 成本计量失真 | 网关单点计量 + requestId 贯穿 + 单价表配置化，禁止散落计算 |
| AI 判定不可解释 | 判定的"决策痕迹/原始输入输出"全量进证据与审计链 |
| 多租户隔离漏洞 | RLS 强制 + 资产级鉴权 + 权限测试用例化 |
| 审计性能 | append-only 分区 + 快照锚点，万亿级再上 Kafka/列存 |
| 前端与后端节奏 | OpenAPI 契约先行，mock 演进为 MSW，前后端并行 |
| 私有化交付 | Go 单二进制 + Docker/Helm + MinIO/TimescaleDB 内网化 |

---

> 评审稿 v0.1 · 2026-09-29 · 供架构评审讨论。定稿后同步 CHANGELOG 与 README。
---

## 8. 评审深化（v0.2）：D1 语言对比 / 资源与任务编排 / D3 工作流 / D4 AI 网关

### 8.1 D1 后端语言对比（Go vs Java/Spring vs Node.js）

> 关键判断：这套系统语言选型不是"写 CRUD 用什么顺手"，而是**执行域（高并发调度/沙箱/真机 farm）+ 私有化交付**决定权重。下表按生产落地视角对比。

| 维度 | Go | Java / Spring Boot | Node.js / NestJS |
|---|---|---|---|
| 并发模型 | **goroutine**：轻量、百万级协程，天然适配"每测试任务一协程"的调度 | 线程池 + 虚拟线程(J21)；虚拟线程接近 goroutine 但生态迁移需 JDK21+ | 单线程事件循环，**CPU 密集(压测/解析)阻塞**，并发要靠多进程/Worker |
| 执行调度重 IO | **极优**：沙箱/真机 farm/事件流是 goroutine + channel 主场 | 好（虚拟线程后），但 JVM 内存/启动开销大 | 一般：I/O 密集尚可，CPU 密集与并发沙箱管理吃力 |
| 内存/资源占用 | 每任务协程开销极小，**沙箱场景省资源** | JVM 基线高（数百 MB+），多任务部署成本高 | 中（单进程省，但并发需多进程放大占用） |
| 私有化交付 | **单静态二进制 + go:embed**，一条命令跑、无运行时依赖 | 需 JRE + 应用服务器；镜像偏大 | 需 Node 运行时 + node_modules；Docker 化勉强 |
| K8s 契合 | 原生（K8s 本身 Go 写）、operator/scheduler 生态 | 成熟但重 | 尚可 |
| 业务 CRUD / ORM | 中等：GORM/SQLC 成熟，样板代码略多 | **最成熟**：Spring Data/JPA/MyBatis，中大型团队最熟 | 好：TypeORM/Prisma |
| 报告导出(模板) | 中 | 成熟 | 好 |
| AI SDK 生态 | 多模型厂商均有 Go SDK；无统一网关则散 | 成熟 | **最全**（LLM SDK 首选 JS 生态） |
| 团队招聘(中大型技术团队) | 中 | **最易**（国内中大型团队 Java 存量最大） | 中 |
| 微服务/框架 | 轻(kratos/go-zero) | 全(Spring Cloud) | 全(NestJS) |
| 长期演进 | 稳，适合"平台型重 IO" | 稳，适合"业务型企业级" | 适合"前端技术栈统一"团队 |

**结论（技术评审立场）**：
- **首选 Go**：因为系统的性能主战场在**执行域（并发调度/沙箱/真机 farm）+ 私有化单二进制交付**，这两项 Go 的收益是决定性的，业务 CRUD 的"样板代码略多"可用模块化 + SQLC 消化。
- **备选 Java(Spring Boot)**：唯一真正合理的替代——当你的团队是 Java 存量为主、招聘以 Java 优先时。此时**保持架构不变**（同上四域划分），用 Spring 实现治理域/可信域，**执行域仍建议独立 Go 服务**补齐高并发沙箱调度。架构决策优先于语言偏好。
- **Node.js 不推荐做主后端**：CPU 密集(压测/契约解析/变异计算)阻塞事件循环，多团队多系统并行的资源编排下并发上限和稳定性风险高；它更适合做**AI SDK 最全的辅助服务**（如契约分析、报告渲染）或网关胶水层。

### 8.2 资源与任务编排架构（多团队多系统并行，系统核心复杂度）

> 定位：这是 GreenPass **最需要先想清楚的架构**——中大型团队多团队并行 + 每个工程独立选模型 + 12 类异构场景，资源编排直接决定"并发上不上得去、隔离靠不靠得住、一个团队会不会拖垮全局"。

**① 资源类型与资源池（统一抽象）**

```
Resource (资源类型, 租户配额, 已占用, 调度状态)
├─ K8s 沙箱      （后端/集成/API/压测：Pod 租借）
├─ 浏览器实例     （Web E2E/视觉/性能：Playwright 集群）
├─ 真机实例       （移动 E2E/兼容/弱网：STF 真机池）
├─ 压测负载       （服务压测/Web 性能：k6/Locust 并发额度）
└─ AI 调用额度    （生成/判定/分析：模型网关并发 + Token 预算）
```

- 每种资源 = **类型 + 额度 + 占用 + 排队**；执行任务声明所需资源（如"1 真机 + 2 沙箱 + 弱网"），调度器按需分配。

**② 多级配额模型（防"单团队拖垮全局"的关键）**

```
配额是分层累加的，任何执行都要在每层校验通过：
  平台级总量配额  →  租户(团队)配额  →  工程(被测对象)配额  →  负责人/成员配额
                           ↑ 工程负责人可按成员/资产精细再分配（PRD R-ORG-5/6）
```

- 每个执行任务占用的资源**同时扣减**其所属团队、工程、负责人的当前余量；
- 团队 A 的并发峰值被团队配额硬顶住，**不会挤占团队 B 的沙箱/真机**；
- 工程负责人分配的"并发/沙箱配额"即为此处的资源配额下钻（PRD 4.3 落地）。

**③ 调度模型（集中式调度器 + 公平调度）**

| 要素 | 设计 |
|---|---|
| 集中调度器 | 单一调度服务持有全局资源视图 + 租户公平调度，避免"各执行器各自抢资源" |
| 队列 | 按 (团队, 优先级, 到达) 入队；任务 = (资源需求, 优先级, 抢占标记) |
| 优先级 | 高：门禁/CI 回写触发；中：人工执行；低：定时/批量；支持优先级抢占(fail-safe) |
| 公平性 | 加权公平队列(WFQ)：防低优团队饿死高优、也防高优常占压垮低优；每团队保底配额 |
| 冲突检测 | 资源互斥(同一真机/同一沙箱不可并发)、同名压测负载互斥；冲突事件上报 |
| 幂等/恢复 | 任务状态机持久化(Temporal)，worker 崩溃后可恢复/重试 |

**④ 执行任务状态机（Temporal 承载）**

```
Queued → VersionCheck → Scheduled → Running → (Paused/Resumed) → CollectEvidence → GateEval → Report → Done
              └─ 失败/超时 → Retry / Failed(可人工重试)
```
- 每步可暂停/恢复（用户"暂停全部/恢复全部"落地）；
- 状态迁移 + 事件全量入审计链（执行即审计素材）。

**⑤ 隔离与多租户安全**
- 沙箱/真机/浏览器实例按租户隔离：Pod 网络隔离(NetworkPolicy)、真机/浏览器实例租户标签 + 配额绑定；**执行器权限最小化**，证据只经签名 URL 上 S3 指定租户桶。

**⑥ 业界对标**
- 云 CI 并发配额（GitHub Actions concurrent / 自托管 runner 池）、K8s ResourceQuota + PriorityClass、Temporal Worker 并发控制。GreenPass 的独特点是把"测试资源(真机/浏览器/沙箱/AI额度)"作为一等资源纳入租户配额体系，而非只做 CPU/内存配额。

> **架构结论**：资源与任务编排 = **集中调度器 + 多级租户配额 + 公平队列 + Temporal 状态机**。这是本平台最该投入设计的地方；建议 P1 阶段就搭出配额与调度骨架，而不是等场景多了再补。

### 8.3 D3 工作流方案可选对比（为什么 Temporal）

| 方案 | 长时任务 | 重试/幂等 | 暂停/恢复 | 可观测/历史 | 自托管(私有化) | 多语言 SDK | 适合度 |
|---|---|---|---|---|---|---|---|
| 自研(状态机+DB) | 可做但全手写 | 手写 | 手写 | 手写 | 是 | — | 三年后为重试/暂停/审计付学费 |
| 消息队列+手动状态(Redis Streams/Kafka+RDBMS) | 可做 | 手写(易漏) | 手写 | 弱 | 是 | — | 只适合极简流水线，不适合 6 步+审核+回退 |
| **Temporal** | **原生** | **原生** | **原生** | **工作流历史即审计** | **自托管** | Go/TS/Java/Py | **最适合** |
| AWS Step Functions | 原生 | 原生 | 有 | 有 | **云锁定** | 云 | 不适合私有化 |
| Camunda/Zeebe(BPMN) | 原生 | 有 | 有 | 有 | 是 | 主 Java | 偏流程审批，重；多语言弱 |
| Airflow | 批处理 | 有 | 有 | 有 | 是 | Py | 面向批调度，不适合在线交互式执行 |
| Celery/BullMQ | 弱(长时需自维护) | 有 | 弱 | 弱 | 是 | Py/JS | 队列型，非工作流 |

**为什么选 Temporal**：
1. **两类核心流程都是"长时 + 多阶段 + 可重试 + 可暂停恢复"**：AI 生成管道（6 步 + 人工审核 + 回退）与执行流水线（版本校验→调度→证据→门禁→报告）——这正是工作流引擎而非队列的用武之地；
2. **工作流历史天然是审计素材**：每步输入输出、重试、暂停记录都在，直接支撑"AI 决策痕迹可追溯""执行即审计"，省掉另写审计采集；
3. **自托管**适配私有化（中大型团队刚需），不云锁定；
4. **多语言 SDK**（Go/TS/Java）允许执行域 Go、治理域 Spring、报告渲染 TS 各自接入同一编排；
5. GitHub Actions 同源思想（Temporal 源自 Uber Cadence，GitHub 也是其用户），业界验证充分。

### 8.4 D4 统一 AI 网关——为什么"正常是必要的"（原因分析）

> 用户判断"统一网关正常是必要的"，这里把**为什么必要**讲透——它不是"好习惯"，而是三条产品需求的**物理前提**：

1. **"每工程可选 AI 模型"必须有路由点**：多模型(豆包/DeepSeek/GPT/Claude/私有) + 按被测对象绑定，必须在**一个入口**按 (团队, 工程, 业务点) 解析该用哪个模型、哪个 key、哪个供应商。没有网关，就是每个服务各写一套路由与密钥，绑定关系散落。
2. **成本审计(总览=Σ明细、存量成本递减可证明)必须以单点计量为唯一真相**：所有 AI 调用的 Token 入/出、单价、金额只在网关统一计量，并把 `requestId` 贯穿到业务点(用例生成/执行/判定)与归属(团队/工程/用例/Run)。否则各服务自算金额，口径必然不一致——这正好打脸 PRD 强调的"成本计算要准确不要混乱"。
3. **可靠性(生产必需)**：统一超时、重试、fallback(主模型挂→备模型)、并发控流、限流——避免生成管道/判定因单模型抖动整体失败。
4. **可观测与治理**：单点采集每次调用(模型/时延/Token/失败/成本)，支撑"AI 决策痕迹"可追溯 + 成本趋势(时序) + 质量判定(哪些模型判错)。
5. **安全与合规**：模型 API Key 单点托管(不散落前端/服务)、模型白名单与审批(企业接入治理，PRD R-MODEL)、私有化/数据出境合规(哪些数据可发给外部模型，网关统一裁剪)。
6. **配额与降级**：AI 调用也纳入资源编排的配额体系(§8.2 的"AI 调用额度")，网关与调度器联动，防一个团队把模型预算烧光。

> **反证**：没有网关 = 每个调用方各自 选模型/存密钥/算钱/重试/记日志，成本审计、模型治理、可靠性、合规四项全部落空，且后期收拢成本极高。**网关不是可选项，是这三条产品线的共同地基。**

---

> 评审深化 v0.2 · 2026-09-29 · 在 v0.1 基础上补 D1 对比 / 资源编排 / D3 方案对比 / D4 原因分析

---

## 9. 可执行落地（v1.0）：Go + Temporal 细化到可实现

> 前置：技术栈已定（**Go + Temporal**，其余按 §3/§8 建议）。本章把方案细化到**可执行程度**——工程骨架、领域包结构、DDL 级数据模型、Temporal 工作流/Activity、API 清单、成本审计数据流、报告链路、前端整合、CI/CD 对接、里程碑。工程落地必须遵守 [ENGINEERING-SPEC.md](./ENGINEERING-SPEC.md)（Go 全量工程规范），UI/交互遵循 [DESIGN-SPEC.md](./DESIGN-SPEC.md)。

### 9.1 Go 工程骨架（对照 ENGINEERING-SPEC）

```text
green-pass/
├── cmd/
│   ├── server/                 # HTTP/API 入口：装配 gateway + 四域 handler
│   └── worker/                 # 异步入口：AI 生成管道 worker + 执行调度 worker + 事件消费
├── internal/
│   ├── gateway/                # 接入：路由/中间件/鉴权/租户上下文/限流
│   │   ├── middleware/         # tenant、authz、audit、request_id
│   │   └── httpx/              # 统一响应/分页/错误编码
│   ├── governance/             # 治理域
│   │   ├── api/                # handler + dto(req/resp) + validator
│   │   ├── application/        # command/query/result + 用例编排
│   │   ├── domain/             # TargetTree/Case/Contract/Report/Team/RBAC/ModelConfig 聚合
│   │   │   └── port/           # 仓储接口 + 域事件
│   │   └── infra/              # GORM 仓储 + PO + RLS 上下文 + 事件发布
│   ├── execution/              # 执行域
│   │   ├── api/ application/ domain/ infra/   # Run/Quota/Job/EnvVersion
│   ├── ai/                     # AI 编排域
│   │   ├── api/ application/ domain/ infra/   # ModelGateway/GenBatch/计量端口
│   ├── trusted/                # 可信域
│   │   ├── api/ application/ domain/ infra/   # GateRule/GateResult/AuditChain/CostAttribution
│   └── platform/               # 平台支撑：rls / observability / config / errors / paging
├── pkg/                        # 无业务纯技术：protocol、clock、hashchain、id
├── api/openapi.yaml            # 唯一前后端契约（生成前端类型）
├── migrations/                 # golang-migrate：000001_*.up/down.sql
├── deploy/ docker/ helm/
├── test/                       # 跨域集成 + e2e（testcontainers 全栈）
├── Makefile                    # make check 统一门禁
└── .github/workflows/ci.yml
```

- **四域边界**：`governance/execution/ai/trusted` 各自 `api→application→domain←infra`；`domain` 零三方依赖（ENGINEERING-SPEC §3.1）。
- **跨域只经契约/端口**：execution 需要模型 → ai 暴露 `ModelGateway` 端口（计量内置）；execution 落审计 → trusted 暴露 `AuditSink` 端口；成本行统一由 trusted 写入（`cost_*` 唯一写者）。
- **Worker 拆分**：`cmd/worker` 注册 Temporal worker（生成管道 + 执行调度）+ 事件消费；`cmd/server` 纯 HTTP。二者共享 `internal/*`，装配差异只在 `cmd/*`。

### 9.2 核心聚合与领域端口（落地样例）

| 域 | 聚合 | 关键行为（domain 方法） | 仓储端口（domain/port） |
|---|---|---|---|
| Governance | `Target`（被测对象树节点） | `AttachRepo` / `SetModelBinding` / `ResolveVersionChain` | `TargetRepository: Find/List/Save` |
| Governance | `Case`（版本化用例） | `CreateVersion` / `MarkChange(added/updated/deleted)` / `RollbackTo(ver)` / `LinkSource(repo+branch)` | `CaseRepository: SaveVersion/FindVersion/History/Rollback` |
| Governance | `Team` / `AssetRBAC` | `AllocateQuota` / `GrantMember(asset, perm, quota)` / `VerifyAccess(actor, asset, action)` | `TeamRepository`, `RBACRepository` |
| Execution | `Run`（测试运行） | `StartRun` / `Pause` / `Resume` / `CollectEvidence` / `ApplyGate` / `ProduceReport` | `RunRepository: Save/Find/ListByTarget` |
| Execution | `Quota`（资源配额） | `Acquire(resources)` / `Release` / `Enforce(team, project, owner)` | `QuotaRepository`（Redis 计数 + PG 账本） |
| AI | `GenBatch`（生成批次） | `Enqueue` / `MarkApproved` / `MarkRolledBack` / `EmitCost(requestId, tokens, unitPrice)` | `GenBatchRepository`, `MeterPort`(→trusted) |
| Trusted | `GateRule` / `GateResult` | `Evaluate(context)`(Rego) / `Decide(block/pass)` / `AnchorAudit(event)` | `GateRepository`, `AuditRepository(append-only)` |
| Trusted | `CostLineItem` | `Record(requestId, bizPoint, model, tokens, amount, owner)` | `CostRepository: Insert/Summarize/CompareHistory` |

### 9.3 数据模型（DDL 级，PG15 + TimescaleDB）

> 表前缀/命名/审计字段遵循 ENGINEERING-SPEC §8；迁移 golang-migrate 只增不改。以下为核心表落库形态。

```sql
-- ========= 治理域 =========
CREATE TABLE tgt_target (                      -- 被测对象树节点（L0 工程/L1 服务组/L2 服务/L3 模块）
  id            BIGINT PRIMARY KEY,
  team_id       BIGINT NOT NULL,               -- RLS 依据
  parent_id     BIGINT,
  level         SMALLINT NOT NULL,             -- 0工程 1服务组 2服务 3模块
  name          TEXT NOT NULL,
  kind          TEXT NOT NULL,                 -- api_service/web_app/mobile_app/contract/ai_model...
  repo_id       BIGINT,                        -- 来源溯源：绑定被测仓库
  model_binding JSONB,                         -- 本工程/节点可选 AI 模型绑定
  status        TEXT NOT NULL DEFAULT 'active',
  created_by/created_at/updated_by/updated_at/deleted_at ...
);
CREATE UNIQUE INDEX uk_tgt_team_parent_name ON tgt_target(team_id, parent_id, name);

CREATE TABLE cas_case (                        -- 用例（版本化主体）
  id BIGINT, team_id BIGINT, target_id BIGINT,   -- 归属被测对象节点
  code TEXT NOT NULL,                            -- 用例编号（按树命名空间稳定）
  title TEXT, kind TEXT,                         -- 场景族：api/web/ui/contract/perf/mobile/weaknet/ai...
  current_version INT NOT NULL,
  status TEXT, ...
);
CREATE TABLE cas_version (                    -- 用例版本（核心：变化可辨）
  id BIGINT, case_id BIGINT, version INT NOT NULL,
  change_type TEXT,                            -- added/updated/deleted（随迭代可辨）
  source_repo_id BIGINT, source_branch TEXT,   -- 来源可溯源
  script_json JSONB,                            -- 脚本/HTTP/规则/压测 + 参数/数据/断言
  approved_by BIGINT, approved_at TIMESTAMPTZ,
  UNIQUE(case_id, version)
);
-- 回退：cas_version 只增不改；RollbackTo 新增一条指向旧版本内容的版本记录（非物理删除）。

-- ========= 执行域 =========
CREATE TABLE run_run (                          -- 测试运行（时间维主键）
  id BIGINT, team_id BIGINT, scenario_id BIGINT, target_id BIGINT,
  target_version TEXT, target_branch TEXT, env_version TEXT,
  run_mode TEXT,                                 -- manual/ci/trigger
  state TEXT,                                    -- queued/version_check/scheduled/running/paused/collect/gate/report/done/failed
  started_at/ended_at, ... 
);
CREATE TABLE run_case_result (                  -- 用例执行结果（成本/证据/历史对比的明细粒）
  id BIGINT, run_id BIGINT, case_id BIGINT, case_version INT,
  status TEXT,                                  -- pass/fail/blocked/skipped/retried
  evidence_ref JSONB,                           -- {screenshots:[s3uri], logs:[s3uri], hash:sha256}
  ai_tokens_in BIGINT, ai_tokens_out BIGINT, cost_amount NUMERIC,  -- 本用例执行成本（单点计量回填）
  attempt_seq INT NOT NULL DEFAULT 1,     -- 同 run 内重试/重跑序号
  UNIQUE(run_id, case_id, attempt_seq)
);

-- ========= AI 编排 + 可信域 =========
CREATE TABLE gen_batch (                        -- 生成批次（关联仓库/分支/版本）
  id BIGINT, team_id BIGINT, target_id BIGINT,
  repo_id BIGINT, branch TEXT, version TEXT,
  state TEXT,                                   -- enqueued/generating/quality_check/review/approved/rolled_back
  parent_batch_id BIGINT,                       -- 回退参照
  -- 同参数允许重新生成（审核驳回/修正重跑），不设全局唯一；并发由生成状态机保证
);

CREATE TABLE cost_line_item (                   -- 成本明细（唯一写者=trusted；append-only）
  id BIGINT, request_id TEXT, idempotency_key TEXT, -- AI 网关 requestId 贯穿 + 幂等键（防重复计量）
  team_id BIGINT, target_id BIGINT, run_id BIGINT, case_id BIGINT,
  biz_category TEXT NOT NULL,                 -- 成本大类：generate(用例生成)/execute(用例执行)——PRD 两大口径
  biz_point TEXT NOT NULL,                    -- 细类：case_generate/case_execute/gate_judge/analysis（归属大类）
  model TEXT, tokens_in BIGINT, tokens_out BIGINT, unit_price NUMERIC, amount NUMERIC,
  occurred_at TIMESTAMPTZ,
  -- 总览=Σ明细；存量执行成本与历史对比可证明（同 case 按 request_id/case_id 对账）
  UNIQUE(idempotency_key)                        -- 防重复落账（同 request_id+biz_point）
);
CREATE INDEX idx_cost_run_case ON cost_line_item(team_id, run_id, case_id);

CREATE TABLE aud_event (                        -- 审计哈希链（append-only，禁 UPDATE/DELETE）
  id BIGINT, team_id BIGINT, actor BIGINT, op TEXT, asset TEXT, asset_id BIGINT,
  payload JSONB, prev_hash CHAR(64), hash CHAR(64), ts TIMESTAMPTZ,
  -- hash = sha256(prev_hash || payload || ts)；周期快照锚点表锚定头尾
);

-- ========= 时序（TimescaleDB hypertable）=========
CREATE TABLE ts_cost_trend (...) ;               -- 成本趋势（连续聚合）
CREATE TABLE ts_exec_history (...) ;             -- 执行历史/通过率趋势
```


-- ========= 治理/执行/可信（P1 落地补齐）=========
CREATE TABLE repo_repo (                        -- 被测仓库（来源溯源：被测对象树 ← 仓库）
  id BIGINT, team_id BIGINT, target_id BIGINT, kind TEXT,
  url TEXT, default_branch TEXT, cred_ref TEXT,  -- 凭据引用，密钥不落库
  created_at/updated_at/...
);
CREATE TABLE repo_branch (                      -- 仓库分支/版本
  id BIGINT, repo_id BIGINT, branch TEXT, version TEXT, head_sha TEXT,
  UNIQUE(repo_id, branch)
);
CREATE TABLE env_runtime (                      -- 测试环境运行版本（"测了没白测"校验依据）
  id BIGINT, team_id BIGINT, target_id BIGINT, env TEXT, running_version TEXT,
  checked_at/updated_at/...
);
CREATE TABLE env_check (                        -- 版本校验记录（目标 vs 环境运行版本）
  id BIGINT, run_id BIGINT, target_version TEXT, env_version TEXT, result TEXT,
  -- result: match / mismatch / unknown；mismatch 阻断执行并入审计链
  checked_at TIMESTAMPTZ
);
CREATE TABLE gate_rule (                        -- 门禁规则（OPA Rego，策略即代码）
  id BIGINT, team_id BIGINT, target_id BIGINT, scenario_id BIGINT,
  rego TEXT, version INT NOT NULL, enabled BOOL NOT NULL DEFAULT true,
  UNIQUE(target_id, scenario_id, version)
);
CREATE TABLE gate_result (                      -- 门禁判定结果
  id BIGINT, run_id BIGINT, rule_id BIGINT, result TEXT, detail JSONB,
  -- result: pass / fail / blocked；判定入审计链
  decided_at TIMESTAMPTZ
);
CREATE TABLE rpt_report (                       -- 测试报告（工程/服务/用例分级 + 跨场景合编）
  id BIGINT, team_id BIGINT, run_id BIGINT, target_id BIGINT, kind TEXT,
  -- kind: project / service / case；跨场景合编=多 run 汇总
  title TEXT, status TEXT, rendered_at TIMESTAMPTZ, export_fmts JSONB
);
**RLS（强制层）**：对 `tgt_*/cas_*/run_*/cost_*/aud_*` 等业务表统一 `CREATE POLICY ... USING (team_id = current_setting('gp.team_id')::bigint)`；资产级 RBAC 在应用层叠加（ENGINEERING-SPEC §8.1）。**审计/成本表只经 trusted 域端口写入，其他域无写权限。** 为在 DB 层强制该约束，PG 用独立角色 gp_trusted_writer 独占 ud_*/cost_* 写权限，gp_server/gp_worker 对审计/成本表仅只读；应用层端口约定仅作第二道防线（对应 ENGINEERING-SPEC §8.1）。

### 9.4 Temporal 工作流 / Activity 划分（两类命脉）

```text
── 工作流 W1：AI 用例生成管道 ──────────────────────────────
Workflow: GenCasePipeline(repo, branch, version, targetId)
  ├─ Activity: FetchSource()          → 拉取仓库代码/文档/契约
  ├─ Activity: ParseAndNormalize()    → 解析出待测实体（契约/接口/页面清单）
  ├─ Activity: AiGenerateSeeds()      → 调模型网关生成用例种子（计量→cost_line_item biz=case_generate）
  ├─ Activity: QualityGate(Rego)      → 种子质量验证（重复/缺失/可执行性）
  ├─ Activity: Review&Approve()       → 人工审核（Temporal Signal 挂起等待）
  ├─ Activity: CommitVersions()       → 按 change_type 写 cas_version
  └─ Activity: EmitAudit()            → 审计链锚定
  （Signal: 审核通过 / 回退 rollback → 参照 parent_batch 撤销未提交版本）

── 工作流 W2：测试执行流水线 ───────────────────────────────
Workflow: ExecuteRun(runId)
  ├─ Activity: ResolveTargetVersion() → 拉取被测对象当时版本/分支/环境版本
  ├─ Activity: VersionCheck()         → 目标版本 vs 环境运行版本 校验（"测了也白测"阻断）
  ├─ Activity: AcquireResources()     → 调度器分配 沙箱/浏览器/真机/AI额度（多级配额）
  ├─ Activity: DispatchScenario()     → 分发到 执行器(沙箱/浏览器集群/真机池/压测负载)
  ├─ Activity: CollectEvidence()      → 截图/日志/脚本 → S3 + sha256；服务级截图开关
  ├─ Activity: EvalGate(Rego)         → 门禁判定（覆盖率/断言/契约/变异/追溯/回放）
  ├─ Activity: AttachCost()           → 本 Run 各 case 执行成本汇总（biz=case_execute）
  └─ Activity: ProduceReport()        → 分级报告（工程/服务/用例）+ 跨场景合编
  （Heartbeat/ContinueAsNew：长时运行分片；Pause/Resume Signal：用户"暂停全部/恢复"）
```

- **执行状态机**（§8.2④）由 Temporal Workflow 承载，状态迁移事件全量入审计链——执行即审计素材。
- **回退**：用例回退不是物理删版本，Temporal 记录 `RollbackSignal` → 生成引用旧版本内容的新版本 + 审计锚定。

### 9.5 API 接口清单（按域，OpenAPI 唯一契约）

| 域 | 资源 | 关键端点 |
|---|---|---|
| Auth/租户 | /auth | `POST /auth/login`(自有账号/微信扫码/手机号绑定)、`POST /auth/wechat/scan`、`POST /auth/refresh` |
| Governance | /targets | `GET /targets`(树+维度筛选联动)、`POST /targets/{id}/repo`(绑仓库/分支)、`POST /targets/{id}/model-binding`、`POST /targets/{id}/version-check` |
| Governance | /cases | `GET /cases`(按树/场景/版本筛选)、`POST /cases`、`POST /cases/{id}/versions`(新版本/change)、`POST /cases/{id}/rollback`、`GET /cases/{id}/history`(版本对比)、`GET /cases/{id}/cost-compare`(与历史成本对比) |
| Governance | /contracts、/reports | `GET /contracts`、`POST /reports`(分级)、`GET /reports/{id}/export?fmt=html|pdf|docx|md` |
| Governance | /teams、/members | `POST /teams/{id}/members`、`POST /teams/{id}/asset-perms`(成员×资产×perm+quota)、`GET /teams/{id}/quota` |
| Execution | /runs | `POST /runs`(含当次用例勾选/树范围筛选)、`POST /runs/{id}/pause|resume`、`GET /runs/{id}`(实时进度 SSE)、`GET /runs/{id}/case-results`、`GET /runs/{id}/evidence` |
| Execution | /scenarios | `GET /scenarios`、`GET /scenarios/{id}`(场景卡片聚焦/任务进行中提示) |
| AI | /generation | `POST /generation/batch`(仓库→版本→生成)、`POST /generation/{id}/approve`、`POST /generation/{id}/rollback`、`GET /generation/{id}/pipeline`(步骤+状态联动) |
| Trusted | /gates | `GET/PUT /gates/rules`(Rego)、`GET /gates/{targetId}/result` |
| Trusted | /cost | `GET /cost/overview`、`GET /cost/items`(生成/执行分大类)、`GET /cost/compare/{caseId}` |
| Trusted | /audit | `GET /audit/events`、`GET /audit/verify/{id}`(哈希链校验) |
| 系统 | /settings | `GET/PUT /settings/**`(仅超级管理员可见的配置) |
| CI/CD | /cicd | `POST /cicd/webhook`(上游推送版本)、`GET /cicd/connector`、`POST /runs` 支持 CI 触发 |

### 9.6 成本审计数据流（总览=Σ明细，口径单点）

```
AI 网关(LiteLLM/自研) ──每次调用──▶ requestId 生成
   │  token 入/出 + 单价 × 用量 = amount
   ▼
trusted.CostService.Record(requestId, bizPoint, model, tokens, amount,
                            team_id, target_id, run_id, case_id)
   ▼ cost_line_item(append-only, 唯一写者=trusted)
总览(按团队/对象/生成vs执行/时间) = Σ cost_line_item     ← 服务端单点计算，前端只展示
明细可下钻：总览 → 团队 → 被测对象 → 用例 → 单次执行(requestId)
历史对比：同 case 的 cost_line_item 按 case_id+时间聚合 → 存量执行成本持平/递减可证明
```

- 前端**不重算**金额（DESIGN-SPEC 状态一致原则延伸）；成本趋势用 `ts_cost_trend` 时序。
- 审计 `cost_line_item` 与 `run_case_result.ai_tokens/cost_amount` 对账校验，防"测了也算不清楚"。

### 9.7 报告生成链路（服务端模板 + 跨场景合编）

```
Report 分级：工程报告 → 服务报告 → 用例级明细；跨场景(API/Web/移动/AI专项)可汇总合编
服务端渲染：HTML/Markdown 直接模板 → PDF(Chromium headless 打印) → Word(docx 模板填充)
证据嵌入：截图/日志经 S3 签名 URL 嵌入报告 + 哈希校验标注
导出：/reports/{id}/export?fmt=html|pdf|docx|md   （不在前端客户端导出，避免易碎）
```

### 9.8 前端整合 DESIGN-SPEC（契约先行 + 状态语义对齐）

- **契约先行**：`api/openapi.yaml` 为唯一契约，前端类型由契约生成（openapi-typescript），后端/前端各自不手写漂移；前端 mock 演进为 Mock Service Worker，与真实 API 同构（§13 ENGINEERING-SPEC）。
- **实时**：执行进度用 SSE/WebSocket（`/runs/{id}` stream）；场景卡片聚焦/任务进行中提示（PRD）由 `run.state + scenario.task_count` 驱动。
- **状态语义映射**（DESIGN-SPEC §2.1 色板 ↔ 后端状态）：`pass→emerald`、`fail/blocked→red`、`pending/partial/queued→amber`、`idle/中性→slate`；主操作/选中只用 emerald；indigo 仅保留分类标识。
- **一页一焦点**（DESIGN-SPEC §1）对应 API 返回分级密度：总览接口轻量、清单接口适中、详情接口全量；前端不自行拼接聚合。
- **密度匹配**：列表接口支持分页 + 查询条件（原型已列出的"列表类页面都要有查询条件"）；后端统一分页/排序契约。

### 9.9 自身 CI/CD 对接（平台加入研发体系规范性）

- **对外三通道**：① `POST /cicd/webhook`（上游 Jenkins/GitHub Actions/流水线推送 `repo+version`，触发"版本→用例生成→执行"联动，PRD 要求联动可后置）；② 官方 **CLI**（`greenpass-cli run --target --version --cases`，供流水线调用）；③ **OpenAPI** 供任意 CI 集成。
- **入站**：CI 触发的 `run` 打 `run_mode=ci`，走最高优先级（§8.2 高优：门禁/CI 回写），配额内抢资源。
- **出站**：执行完向 CI 回写门禁结果（pass/fail + 报告 URL + 证据），支撑"测试质量作为发布门禁"闭环。
- **自身版本契约**：`api/openapi.yaml` 语义化版本；上游 CI 集成按契约版本对接，破坏性变更走兼容窗口（对齐 ENGINEERING-SPEC §12 发布）。

### 9.10 脚手架与里程碑（可执行 P0–P4）

| 阶段 | 可执行目标 | 关键产出（遵循 ENGINEERING-SPEC + DESIGN-SPEC） |
|---|---|---|
| **P0 工程骨架** | 仓库/CI/门禁就绪 | `cmd/server+worker`、四域包、`api/openapi.yaml` 骨架、`Makefile check`、migrations 基线、testcontainers、RLS 脚本、GitHub Actions |
| **P1 最小闭环** | 单工程跑通"测了没白测" | 被测对象树+绑仓库+版本校验、用例版本化(change/回退)、1 场景执行(API/K8s沙箱)、门禁(OPA)、报告导出(HTML)、成本明细(生成+执行)落 `cost_line_item` |
| **P2 异构执行** | 多场景 + 调度 | K8s 沙箱/浏览器集群/真机池接入、Temporal W2、多级配额/公平队列、证据(截图开关)、Pause/Resume、跨场景报告合编 |
| **P3 AI 治理** | 平台化 | 统一 AI 网关(LiteLLM→自研)、Temporal W1 生成管道、成本趋势(时序)+历史对比、审计哈希链锚定、模型治理/白名单 |
| **P4 平台化** | 中大型团队 | 资产级 RBAC 精细化、CI/CD 对接(webhook/CLI/回写)、微信/SSO 登录、私有化交付、审计链独立校验 |

**首批可运行边界（P1 验收）**：一个被测对象 → 绑仓库/分支 → 生成用例(带 change) → 版本校验 → 执行一个 API 用例 → 截图证据 → 门禁 → HTML 报告 → 成本明细可下钻可对比。**口径自洽（总览=Σ明细、版本可溯源）为硬验收。**

---

## 10. 水平扩展与多团队高并发架构

> 定位：GreenPass 面向中大型技术团队、多团队多系统并行测试。系统实现后应能**通过横向扩展节点（加副本 / 加 worker / 加执行集群）线性支撑团队数增长**，而非靠单机或改单体。本节为系统的扩展性设计，落地基线见 §9，里程碑映射见 §10.9。

### 10.1 扩展性原则（无状态优先 + 有状态分层）

- **控制面（HTTP server / 调度决策 / worker 逻辑）一律无状态**：不落进程内可变业务状态，可任意加副本、随时重启、被 LB 调度。
- **有状态只出现在明确边界**：PG（权威）、Redis（缓存/队列/配额）、Temporal（工作流/任务）、MinIO（对象）。业务代码不得持有跨请求状态。
- 共享可变状态只经 Redis / Temporal / PG，**不用进程内单例或本地内存做跨副本共享**。
- 每类节点独立扩缩：控制面、worker、执行沙箱按各自瓶颈扩，互不阻塞。

### 10.2 控制面（server）水平扩展

- server 无状态（JWT 鉴权、不存本地会话；幂等由请求幂等键 + DB 保证）。
- 多副本 + 负载均衡：K8s Deployment + Service（ClusterIP/LB）+ ingress-nginx 或 HPA（按 QPS/CPU 扩缩）。
- 会话态（如有）进共享 Redis；文件上传走直传 MinIO（签名 URL），不走 server 中转。
- SSE 实时推送多副本注意连接归属：用共享订阅（Redis Pub/Sub 或事件总线）广播，单副本连接归属不阻塞扩展。

### 10.3 worker 水平扩展（Temporal 多实例）

- `cmd/worker` 是**无状态消费者**：同一 Temporal Task Queue 可注册多个 worker 实例 → **天然横向并行**消费生成/执行任务。
- 生成管道 W1 / 执行流水线 W2 的 Activity 均无状态（幂等），worker 可随队列深度扩缩。
- 事件消费（Redis Streams）用 Consumer Group 分片，多消费实例并行。
- worker 只依赖"共享工作源"（Temporal / Redis），不落进程内队列。

### 10.4 集中调度器多实例化（关键瓶颈，不能单点）

- §8.2 的"集中调度器"不是单进程：**调度决策无状态化**，决策所需状态（配额余量、资源占用、队列）放 Redis/DB，多调度器实例竞争同一队列。
- 队列用 **Redis Streams Consumer Group**（或 Redis 原子 Lua）：多实例并发消费 + 断点续读，天然分片，支持横向加实例。
- 配额/计数原子性：Redis 原子操作（`DECRBY`+校验）或 Lua 脚本扣减与回补，多实例并发安全；账本落 PG 供审计。
- 公平队列（WFQ，多租户/工程/资产分级）在 Redis 内实现，多实例并发消费不破坏公平性（按 key 分片 + 权重）。
- 冲突检测 / Pause-Resume 状态存 Temporal（工作流即状态机），调度器只做"准入 + 分发"。
- 结论：调度器按无状态副本横向扩展，单点风险消除，瓶颈转移到 Redis / Temporal（两者均可集群化）。

### 10.5 基础设施水平扩展

| 组件 | 扩展方式 | 触发点 |
|---|---|---|
| PostgreSQL | 主从（读副本 offload 查询/报表）；审计/时序走 Timescale hypertable 自动分区；量级再大 → schema-per-tenant 或独立实例 | 读放大/团队数增长 |
| Redis | 单实例 → Redis Cluster（slot 分片）/ Sentinel；队列/配额/缓存分离实例 | 高并发队列/配额 |
| Temporal | 原生多节点集群（frontend/history/matching 横向扩展） | 工作流/任务规模 |
| MinIO | 分布式（erasure coding，多节点） | 证据对象增长 |
| Timescale | 连续聚合 + 分区 + 下采样 | 成本/执行趋势增长 |

### 10.6 多团队/多租户数据扩展

- 基线 RLS 共享库（team_id 行级隔离）；RLS 保证"加团队不加实例"也安全。
- 大团队/大客户 → **schema-per-tenant 或独立集群**（§8.1 已留）：表结构一致，仅数据位置迁移。
- 数据分区策略：按 team_id 哈希或业务主键分区（表分区/分库），查询路由在数据访问层做。
- 配额隔离（§8.2 多级配额）保证一个团队/工程不能拖垮全局——这是"多团队高并发"的第一道闸。

### 10.7 写放大治理（审计 / 成本 / 时序高频写）

- 审计 `aud_*` append-only：分区表 + 快照锚点；超高量级再走 Kafka + 列存（§7 已列）。
- 成本 `cost_line_item` / 时序 `ts_*`：Timescale hypertable 自动按时间分区；连续聚合做趋势。
- 高频写缓冲/批写：证据元数据、执行心跳可先入 Redis 缓冲再批量落库，避免单点写放大。

### 10.8 执行面扩展节点（多执行集群）

- 执行沙箱（K8s Job / 浏览器 / 真机池）是**独立执行节点/资源池**，作为可加节点横向扩展（§8.2 多执行集群）。
- 接入点：GP 为每个执行集群/资源池登记"执行节点"，调度器按配额把任务分发到对应节点。
- 私有化 = 单节点；SaaS = 多执行集群随团队/负载加节点；执行节点与主控制面故障域隔离。
- 沙箱网络（NetworkPolicy 仅出向被测服务）随执行节点复制，隔离保持一致。

### 10.9 从"单体优先"到"多节点"的演进路径（与 P0–P4 映射）

| 阶段 | 部署形态 | 扩展动作 |
|---|---|---|
| P1 | 单副本（1 server + 1 worker） | 按 §9 落地，无状态代码为多副本埋点 |
| P2 | server 多副本 + worker 多实例 | 无状态化 + 共享 Redis/Temporal；调度器无状态化 |
| P3 | 执行域独立服务 + 多执行集群 | 按域拆独立 cmd；沙箱/浏览器/真机作独立执行节点 |
| P4 | 多团队 + 多租户分区 | schema-per-tenant、基础设施集群化、按需扩节点 |

> 关键：**从 P1 起代码即无状态**（多节点扩展的前提，不是后补）；P2 加副本即可线性扩容，无需改架构。

### 10.10 扩展性验证

- 压测基线：多团队并发执行（如 N 团队 × M 工程 × 并发用例），验证 QPS / 队列深度 / 时延 / 配额隔离。
- 副本扩容验证：加 server/worker 副本后吞吐线性上升；缩容安全。
- 故障域验证：停一个调度器 / worker / 执行节点，队列不断、任务由其他实例接管。
- 多租户隔离验证：一个团队占满配额，其他团队不受影响（延续 §8.2 硬验收）。

---

> 可执行稿 v1.1 · 2026-09-29 · 技术栈 Go+Temporal 定稿；工程规范见 ENGINEERING-SPEC.md，UI/交互见 DESIGN-SPEC.md，扩展性见 §10。