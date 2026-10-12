# GreenPass · GP4 平台化实施规格（草案）

> 状态：待决策、待开发。本文把 `IMPLEMENTATION-PLAN.md` 的 GP4-01～GP4-06 从任务级施工图细化为可拆分开发包；不改变既有实施计划，也不宣称任何生产集成已经完成。
>
> 依据：`PRD.md` R-ORG-1～6、R-AUTH-01、R-CICD-01；`TECH-DESIGN.md` §9.5、§9.9、§10；`ENGINEERING-SPEC.md` §2、§8、§9、§12、§13。对没有定稿的产品或基础设施选择一律以 **[用户决策]** 标记。

## 0. 现状与共同约束

### 已有基础

- 租户：所有现有业务表均以 `team_id` + PostgreSQL RLS 做租户隔离；HTTP 目前仅解析 `x-gp-team-id` 与可选 `x-gp-user-id`。该头部不是可信身份凭证。
- 执行：GP2 已具备本地配额、运行队列、资源池和冲突检测参考实现；跨进程 Redis / 多 worker 真实验证仍未完成。
- 可信：`aud_event` 为 append-only 哈希链，门禁、成本、报告已有 trusted 域服务；尚无审计锚点或独立校验程序。
- 部署：当前仅有 kind 命名空间、NetworkPolicy、资源配额和 Temporal values；没有 Helm chart、Ingress、生产 Secret、监控栈或私有化安装包。
- 前端：团队、登录、CI/CD、系统设置页面仍是 mock 交互；`api/openapi.yaml` 尚未定义 `/auth`、`/teams`、`/members`、`/cicd`、运维接口。

### 不可突破的工程边界

1. `api/openapi.yaml` 是唯一外部 HTTP 契约；新端点必须先更新 OpenAPI、生成前端类型，再实现服务端与 MSW。
2. 保持 `api -> application -> domain <- infra`；领域层零第三方依赖；HTTP/JWT/OIDC/PG/Redis/Kubernetes/Temporal SDK 仅在 API、infra 或 `cmd` 装配处出现。
3. RLS 继续作为强制租户边界；身份声明只能从已验证凭证生成，不能继续由客户端头部直接决定。审计、成本仍只能经 trusted 写入端口和 DB 权限写入。
4. 迁移只能新增。`iam_`、`auth_`、`cicd_`、`ops_` 等表前缀需要在 ENGINEERING-SPEC 的登记表中先补齐；不能把核心查询字段藏在 JSONB。
5. 所有权限拒绝、凭证变更、CI 入站/回写、租户路由切换、部署/迁移以及告警确认均要产生 trusted 审计事件；日志不得写 token、手机号、Authorization、Webhook 正文或完整 OIDC claim。

### 必须先由用户决定的横向事项

| 决策 | 影响 GP | 必须明确的最小问题 |
|---|---|---|
| 身份源 | 01/03 | 自建账号是否保留为生产登录；企业 SSO 首批支持 OIDC、SAML 还是两者；微信开放平台主体/回调域名是否具备；手机号短信供应商与合规边界。 |
| CI 首批范围 | 02 | 首批必须联调的 CI 平台（Jenkins、GitLab CI、GitHub Actions、自研中的哪些）；入站事件类型；回写状态模型；是否要求 GitHub Check / GitLab commit status。 |
| 交付拓扑 | 04/05 | 私有化是 air-gapped 还是可出网；支持 Kubernetes 版本/发行版、外置或内置 PG/Redis/Temporal/对象存储、TLS/证书来源、升级/回滚窗口。 |
| 租户分区策略 | 05 | 触发 shared-RLS → schema-per-tenant / 独立库的客观阈值；客户是否允许迁移停机；跨租户运维查询是否需要。 |
| SLO 与责任 | 06 | 目标并发、P95/P99、可用性、恢复目标、压测容量档位、告警接收人和通知渠道。 |

## 1. GP4-01 资产级 RBAC 精细化

### 目标与范围

将 PRD 的「团队角色 + 成员 × 被测对象 × 权限级别 + 并发/沙箱配额」由前端 mock 和当前可伪造的用户头，落实为服务端强制授权。首批权限值严格限于现有 PRD：`full`、`edit`、`exec`、`view`、`none`；团队角色为 `owner`、`admin`、`tester`、`viewer`。资产继承规则、拒绝优先规则、跨层覆盖策略尚未在 PRD 定义，必须先确认，不能自行假定。

### 前置决策与外部依赖

- **[用户决策]** 资产权限对 L0～L3 被测对象的继承语义：只精确匹配、父节点向子节点继承，还是两者并存；是否允许子节点显式 `none` 覆盖父节点。
- **[用户决策]** 角色矩阵是否为固定内置配置，还是 owner 能自定义角色/权限；本规格先按固定四角色实现，避免引入未定义的权限 DSL。
- **[用户决策]** 工程负责人是仅 L0 工程必填，还是任意被测对象都可设置；负责人变更是否需要双人确认。
- 依赖 GP4-03 的认证主体，但可以先用测试用已验证 Principal 端口完成领域、application、API 授权测试；不能以客户端 `x-gp-user-id` 作为生产身份。

### OpenAPI 与事件契约

在既有全局 team header 之外，鉴权由 bearer credential 解析，禁止由请求体传 actor/team 作为授权依据。拟新增资源如下；字段和错误码定稿前先写进 `api/openapi.yaml`：

| 端点 | 目的 | 幂等/授权 |
|---|---|---|
| `GET /teams/{teamId}/members` | 成员列表与状态 | `owner/admin`；team 必须等于 credential 允许范围 |
| `POST /teams/{teamId}/members` | 邀请/添加成员 | `owner/admin`；需要 invite 标识或身份已存在规则 **[用户决策]** |
| `PATCH /teams/{teamId}/members/{memberId}` | 改团队角色、停用/恢复 | `owner/admin`；禁止最后一个 owner 被降级/停用 |
| `GET /targets/{targetId}/access` | 查询当前资产授权分配 | `full` 或 `admin/owner` |
| `PUT /targets/{targetId}/access/{memberId}` | 设置一名成员的 asset permission 与配额 | `full`（负责人）或 `admin/owner`；`Idempotency-Key` 必填 |
| `DELETE /targets/{targetId}/access/{memberId}` | 撤销显式权限 | 同上；不得删除唯一负责人而无替代者 |
| `GET /me/permissions?target_id=` | 前端按当前身份显示可用动作 | 已登录成员；仅返回有效权限，不泄漏其他成员配额 |

建议的版本化内部事件（不是 HTTP DTO）为 `iam.member.changed.v1`、`iam.asset-permission.changed.v1`、`iam.asset-owner.changed.v1`。事件 envelope 至少包含 `event_id`、`occurred_at`、`team_id`、`actor_id`、`asset_id`、`subject_id`、变更前后摘要和 `request_id`；敏感属性不进入 payload。每个变更同步 append 到 `aud_event`。

### 迁移与数据模型

新迁移建议按独立小包拆分；表名以 **待登记** 的 `iam_` 前缀命名：

- `iam_member`：`id`、`team_id`、`principal_id`、`role`、`status(active|disabled|invited)`、`last_active_at`、审计字段；`UNIQUE(team_id, principal_id)`，索引 `(team_id, status)`。
- `iam_asset_owner`：`team_id`、`target_id`、`member_id`、`assigned_at`；`UNIQUE(team_id, target_id)`。若允许多负责人须先改变 PRD 语义。
- `iam_asset_perm`：`team_id`、`target_id`、`member_id`、`permission`、`concurrent_quota`、`sandbox_quota`、`version`、审计字段；`UNIQUE(team_id, target_id, member_id)`。`quota` 单位和 `NULL/0`（无上限/禁止）的语义必须定稿。
- `iam_permission_audit` 不另建第二条审计链；权限变更写 `aud_event`。若需要权限历史查询，可建只追加投影表，但其权威性仍是 `aud_event`。

所有表启用 RLS、创建 INSERT 时 team trigger、查询索引。`principal_id` 的真实主体表归 GP4-03；在其完成前可将外键约束延后，但不可将外部 IdP subject 直接当业务用户 ID 混用。

### 分层职责

- `governance/domain`：成员、资产授权值对象及不变量；`AccessPolicy` 端口和「角色基线 ∩ 资产权限 ∩ 配额」决策模型。领域层只接受明确 `Actor`，不解析 token。
- `governance/application`：成员/授权命令、事务、负责人及最后 owner 保护、调用权限决策、发出 trusted 审计端口、把 per-member 配额同步/查询到 GP2 配额准入端口。
- `governance/infra`：PG `iam_` repository、缓存失效/版本化读取；如使用缓存，失效事件必须强一致优先或有可证明的短 TTL 回退。
- `governance/api`：DTO 校验、调用 application；只返回 Result DTO。
- `gateway`：从 GP4-03 认证中间件取 Principal，路由/动作映射为 `Action`，调用授权中间件或各 application 的显式授权门；禁止在 handler 旁路检查。
- `execution`：仅消费已经授权的配额上限/owner ID，GP2 Redis quota 是执行准入事实来源；RBAC 不直接改写 Redis 计数。

### 验收与依赖

- domain：角色矩阵、继承/覆盖决策、负责人不可被意外撤销、配额边界表驱动测试。
- application：owner/admin/full/edit/exec/view 的行为矩阵；跨团队对象返回不可见；变更会审计；并发更新借助 `version` 或事务锁不丢失。
- infra/api：RLS 隔离、401/403/404 的既定不泄漏语义、OpenAPI 契约、从前端生成类型调用。
- e2e：`exec` 权限仅能创建/控制被授予资产的 run；个人配额与 GP2 team/target/owner 多级配额一起生效。
- 依赖：GP2-03 的共享配额实现、GP2-08 资源池持久化完成后才可做真实配额端到端验收。GP4-03 完成后替换测试 Principal 为真实身份。可先编码领域、迁移、授权门与 fake 测试。

## 2. GP4-02 CI/CD 对接成熟化

### 目标与范围

把技术方案中的 webhook / CLI / OpenAPI 三通道落实为可认证、可重放防护、可幂等、可回写的 CI 集成闭环：上游携带仓库版本触发测试，产生 `run_mode=ci` 的高优 run，门禁终态后将不可伪造的结果、报告 URL、证据摘要回写。首批只实现用户指定的 CI 平台；通用 webhook 契约不能冒充已适配所有 Jenkins/GitLab/GitHub 行为。

### 前置决策与外部依赖

- **[用户决策]** 首批平台和验收环境；每个平台的签名/Token、回写 API、commit/merge request/check 语义不同。
- **[用户决策]** 触发事件（push、tag、MR/PR、手动 pipeline 等）、分支/路径过滤、失败策略（平台不可回写时 fail-open 或 fail-closed）及重试上限。
- **[用户决策]** Webhook Secret、CLI client credential 的存储方式与轮换 SLO；需要接入现有 Secret 管理或 K8s Secret。
- 依赖真实 GP2 队列/配额/资源池和 run 终态事件；否则只能完成适配层单测，不能声称 CI 执行闭环可运行。

### OpenAPI、CLI 与事件契约

建议将每种外部 provider 固有 webhook 留在 adapter 路由，通过通用 Command 收敛；对外稳定接口分两层：

| 接口 | 关键字段 / 约束 |
|---|---|
| `POST /cicd/connectors`、`GET/PATCH /cicd/connectors/{id}` | provider、display_name、enabled、credential_ref（不得返回 secret）、callback policy、允许目标/仓库范围；仅 owner/admin。 |
| `POST /cicd/connectors/{id}/webhook` | 原始请求体用于 provider 签名验真；headers 不进入普通日志；事件 delivery ID + provider + connector 形成幂等键。 |
| `POST /cicd/runs` | 面向官方 CLI/OpenAPI 的标准触发；`target_id`、`version/commit`、`branch`、`scenario_ids/case_ids`、`env`、`callback_ref`、`idempotency_key`；server 从 connector/credential 推导 team，禁止请求方指定他人 team。 |
| `GET /cicd/runs/{id}` | 返回 provider correlation、内部 `run_id`、当前状态、门禁状态、报告引用；权限同 run。 |
| `POST /cicd/runs/{id}/cancel` | 用户决策是否允许上游取消正在执行的 run；若允许必须映射 GP2 Pause/Cancel 语义。 |

CLI 应作为 versioned client（例如 `greenpass run` / `greenpass wait`），使用标准 API，不直连数据库或 Temporal。CLI 输出机器可读 JSON 和人可读摘要的字段、退出码（pass/fail/blocked/platform-error）属于 **[用户决策]**，须为 CI 脚本稳定承诺。

内部事件：`cicd.delivery.received.v1`、`cicd.trigger.accepted.v1`、`run.completed.v1`、`gate.decided.v1`、`cicd.callback.delivered.v1` / `failed.v1`。回写采用 outbox + worker，回写 request 有独立幂等键，不在 run 请求线程中同步等待。

### 迁移与数据模型

建议新增并登记 `cicd_`：

- `cicd_connector`：team、provider、配置版本、secret/credential reference、允许范围、状态；secret 本体不入 PG。
- `cicd_trigger_rule`：connector、target、事件/分支/路径过滤、scenarios、environment、enabled。
- `cicd_delivery`：connector、provider_delivery_id、payload_digest、received_at、verification state、correlation ID、处理状态；`UNIQUE(connector_id, provider_delivery_id)` 或明确无 delivery ID 时的 hash + 时间窗策略 **[用户决策]**。
- `cicd_run_link`：delivery/手动请求、run_id、外部 pipeline/job/commit 标识、回写状态与最后错误摘要；`UNIQUE(connector_id, external_run_id)` 与请求幂等键。
- `cicd_callback_outbox`：目标 URL reference、签名 key reference、payload digest、attempt、next_retry_at、状态。正文不可明文持久化，或须定义加密/保留期。

每次验签失败、重复投递、触发接受、回写结果均 append 到 trusted 审计。payload 保存策略、脱敏字段及保留期是 **[用户决策]**。

### 分层职责

- `governance/domain` 或新增独立 `integration` 限界上下文：连接器、触发规则、delivery 幂等状态；归属必须先通过 ADR，避免将 CI provider SDK 混入 execution/domain。
- application：验签后的标准 TriggerCommand、规则匹配、创建 run、写 outbox；调用 execution 端口而不直连其表。
- infra：provider webhook verifier、外部 API callback client、Secret reference resolver、outbox repository/worker。
- gateway/api：限制 body 大小、读取原始 body 完整用于 HMAC，防重放；验证后才解析 JSON。外部 webhook 走 connector 级凭证，不沿用普通浏览器 CORS / session 认证。
- execution：CI run 使用 `run_mode=ci`，优先级规则必须接 GP2 WFQ/Redis 事实实现，且不得绕过配额、版本校验、冲突检测或门禁。

### 验收与依赖

- provider 合同测试：签名正确/错误、重复 delivery、乱序、过期时间戳、payload 大小、回调 2xx/4xx/5xx 重试。
- run 契约测试：CI 触发不允许任意 team/target；相同 delivery 只建一个 run；报告/门禁终态回写最多一次（或确定的幂等重试）。
- CLI：无凭证、凭证无权限、queued/pass/fail/blocked、网络中断恢复的退出码与 JSON 输出测试。
- kind e2e：使用 provider sandbox/本地 webhook fixture 触发真实部署的 run，验证回写；该项依赖 GP2-01、03、04、07、08 的真实共享基础设施与可执行资源。

## 3. GP4-03 微信 / SSO 登录

### 目标与范围

替换当前只透传 `x-gp-user-id` 的占位中间件，实现「自有账号 + 手机号 + 微信扫码 + 企业 SSO」到统一 Principal 的认证和账号绑定。实现前必须确认具体身份源和法务/隐私要求；没有微信开放平台资质、回调域名、短信服务供应商或企业 IdP 元数据时，不能实现真实登录，只能实现 provider-independent 端口和本地 fake。

### 前置决策与外部依赖

- **[用户决策]** SSO 协议/首批 IdP：OIDC、SAML，或两者；issuer、客户端注册、redirect URI、scope/claim 映射、组织/团队映射。
- **[用户决策]** 微信登录用网站扫码、开放平台还是企业微信；回调域名、主体、UnionID/OpenID 绑定准则及解绑流程。
- **[用户决策]** 手机号登录供应商、地区、验证码频率/有效期、反自动化、人机验证、未注册是否自动建账号（PRD 原型如此描述，但生产需确认）。
- **[用户决策]** 自有账号密码策略、MFA、恢复/账号合并、注销和个人数据保留策略。
- **[用户决策]** 会话模型：短期 JWT + refresh token、服务端 session，或接入既有企业 identity gateway；决定 token 吊销、跨设备登出和 Redis 依赖。

### OpenAPI 与事件契约

不得按原型硬编码 `/auth/login` 的粗略语义，需按授权流拆分。候选资源：

| 端点 | 目的 |
|---|---|
| `POST /auth/password/login` | 密码凭证换取 session/token；是否开放注册另行决定。 |
| `POST /auth/phone/challenges`、`POST /auth/phone/verify` | 申请/校验短信挑战；响应永不透露手机号是否已注册。 |
| `POST /auth/wechat/scan-sessions`、`GET /auth/wechat/scan-sessions/{id}` | 创建二维码会话/轮询状态；真正 provider callback 为单独后端回调 URL。 |
| `GET /auth/sso/{provider}/start`、`GET /auth/sso/{provider}/callback` | OIDC authorization-code + PKCE 或 SAML 重定向；协议选择后只实现对应端点。 |
| `POST /auth/refresh`、`POST /auth/logout`、`GET /auth/me` | 刷新、撤销、当前主体和团队 membership/当前团队。 |
| `POST /auth/identities/{id}/unlink` | 解除手机号/微信/SSO identity；禁止解除最后一种可登录 identity。 |

Authorization bearer token 应包含稳定 `principal_id`、认证时间、issuer、token id、会话版本；`team_id` 不应成为唯一永久 claim，当前团队需要 membership 校验。需要定义 token 返回形态、cookie vs header、CSRF/CORS 策略 **[用户决策]**。内部事件采用 `auth.login.succeeded.v1`、`auth.login.failed.v1`、`auth.identity.linked.v1`、`auth.session.revoked.v1`，写入可信审计且不得记录验证码、token、完整手机号或 provider code。

### 迁移与数据模型

建议新增并登记 `auth_`：

- `auth_principal`：平台主体，不与 team 绑定；状态、display_name、审计字段。
- `auth_identity`：principal、provider、provider_subject_digest/加密列、verified_at、metadata（非核心扩展）；`UNIQUE(provider, provider_subject)` 的可索引安全表示需要安全设计 **[用户决策]**。
- `auth_session` / `auth_refresh_token`：仅保存 token hash、session version、过期、撤销时间、client metadata 摘要；不保存 access token 明文。
- `auth_login_challenge`：短信/扫码/SSO state/nonce/PKCE verifier 的短期状态，TTL + 一次性消费；若放 Redis，PG 只留必要审计投影。
- `auth_sso_provider`：issuer/metadata URL/client credential reference/claim mapping/启用状态；私密 client secret 使用 Secret reference。

团队成员表通过 `principal_id` 关联，不能把手机号、UnionID 或 SSO subject 复制进 `iam_member`。所有实际 identity 表是否使用 RLS，取决于「一个 principal 可跨团队」的产品模型；必须在迁移前由用户确认。至少对自助接口按 Principal 级访问控制，系统管理员读取需审计。

### 分层职责

- 可新建 `internal/identity` 限界上下文（推荐）或经 ADR 放在 `governance`；不要把身份生命周期塞进 gateway middleware。domain 定义 Principal、Identity、Session、认证/绑定不变量与端口。
- application 编排登录、挑战、回调、绑定、session 轮换、成员/团队选择，并经 trusted 写审计。
- infra 实现密码 hash、OIDC/SAML client、微信/短信 adapter、token signer/session store、rate limiter。密码哈希/第三方 SDK 只在 infra。
- gateway Auth middleware 验证 bearer/session，填入可信 Principal + request id；Tenant middleware 只允许 membership 允许的 team，而非相信请求头。过渡期的开发测试头必须在非 production 配置中显式启用，且绝不能默认启用。
- front-end 使用 OpenAPI 生成类型；避免 localStorage 长期存储高权限 token，具体 cookie/BFF 方案取决于用户决策。

### 验收与依赖

- domain/application：身份不可重复绑定、挑战单次消费/过期、刷新 token 轮换与重放拒绝、解绑最后登录方式拒绝、停用成员立即拒绝。
- API/security：JWT/session 验证、issuer/audience/nonce/PKCE、CSRF、redirect allowlist、rate limit、错误信息不枚举账号、敏感日志扫描。
- provider contract/e2e：测试 IdP + 微信/短信 sandbox（用户提供）完成三种登录流；成功后进入正确团队、权限按 GP4-01 生效。
- 依赖：GP4-01 的 `iam_member` 可先依赖临时 Principal port；真实全链认证必须先于任何「前端 RBAC 强制」或 CI human token 发放。

## 4. GP4-04 私有化交付

### 目标与范围

交付一个可离线或受限网络环境安装、升级、验证与支持的 GreenPass 发行物：应用静态二进制/前端资源、版本化 migrations、Helm chart、依赖拓扑、配置/Secret 说明以及审计链独立校验工具。当前仓库没有 Helm chart，因此此任务不能仅补一个 README 即称完成。

### 前置决策与外部依赖

- **[用户决策]** 支持的 Kubernetes 发行版/版本、CPU 架构、是否必须 air-gapped、是否允许 Helm 拉取公网 chart/image。
- **[用户决策]** PG/Timescale、Redis、Temporal、SeaweedFS 是内置子 chart、客户自带，还是两种均支持；备份、证书、存储类、镜像仓库、升级责任归属。
- **[用户决策]** 交付许可/激活是否在范围内；若不在范围，不建立伪造 license service。
- **[用户决策]** 是否要求单二进制运行在非 Kubernetes 主机；若要求，需要单独定义依赖管理、TLS、systemd 和数据目录，不能将 Helm 方案硬套。

### 契约、管理接口与事件

外部业务 OpenAPI 保持不变；新增管理接口只在已认证的 cluster admin/owner 范围可见，避免泄漏基础设施拓扑：

- `GET /healthz`：liveness，只表明进程存活。
- `GET /readyz`：readiness，明确依赖的 DB/Redis/Temporal/对象存储状态，但响应不含 DSN/凭证。
- `GET /version`：应用版本、OpenAPI 兼容版本、迁移版本、build metadata。
- `POST /audit/verify` 或独立 CLI：接收/读取范围，返回 hash-chain 验证结果和锚点引用；需与 GP3/GP4 审计锚定规格对齐。

安装/升级不是 HTTP 业务 API。应产生 `ops.install.validated.v1`、`ops.migration.applied.v1`、`ops.backup.restored.v1` 等运维审计事件；何种系统身份写入由 trusted 管理端口决定。

### 交付物、迁移与分层职责

- `deploy/helm/gp/`：Chart、values schema、server/worker Deployment、Service、Ingress（可选）、NetworkPolicy、HPA/PDB（取决于拓扑）、Job migration、ServiceAccount/RBAC、Secret 引用。不得把密码放 values 默认值。
- `deploy/images/` 或构建流水线：固定 Go toolchain、多阶段构建、`go:embed` 包含前端产物与可执行 migration assets；镜像 tag 与 app/OpenAPI 版本可追溯。
- `cmd/migrate` 或受控 Helm hook Job：只执行已编译/已挂载的 migrations，迁移锁、失败退出和回滚 runbook 明确；绝不在 server 请求路径 AutoMigrate。
- `cmd/audit-verify`：独立读取最小权限 DB 凭据或导出文件，验证 hash 链和后续锚点；不得与 server 共享 writer 权限。
- 文档：`deploy/README`、values reference、air-gap 镜像清单、依赖矩阵、安装/升级/回滚/备份恢复/故障排查 runbook。

私有化本身不增加业务表；若需要 schema version、安装 ID、运营投影，表前缀和是否属于可信审计必须通过 ADR 和迁移登记后再加。

### 验收与依赖

- 离线/目标网络的干净集群安装：image pull、Secret 注入、migrations、readiness、最小业务闭环均成功。
- 升级：支持的 N→N+1 版本执行只增迁移、前后 API/worker 兼容、失败可按 runbook 恢复；恢复演练验证 RPO/RTO（指标需用户定）。
- 安全：默认 NetworkPolicy、非 root/只读文件系统（若运行依赖允许）、最小 K8s RBAC、无敏感 values/日志、可信域写权限隔离。
- 依赖：GP2 真实依赖服务、GP3 审计锚定/独立校验、GP4-03 Secret/认证模型。可以先开始 chart 骨架、values schema、构建与 install smoke；不应依赖尚不存在的真实执行集群去宣称完成。

## 5. GP4-05 多租户分区 + 基础设施集群化

### 目标与范围

在既有 shared database + `team_id` RLS 模型之上，提供可逆性受控的升级路径：普通团队共享库，符合阈值的大客户迁入专属 schema 或独立数据库/集群；PG、Redis、Temporal、对象存储按真实容量和故障域扩展。此项不是「把所有 Helm 副本调大」：数据路由、迁移编排、备份恢复、可观测与运行一致性必须完整。

### 前置决策与外部依赖

- **[用户决策]** 采用 schema-per-tenant、database-per-tenant、cluster-per-tenant 中的哪一层及触发阈值；TECH-DESIGN 只列出候选，未定稿。
- **[用户决策]** 控制面是否始终共享；Temporal namespace 是否按 tenant 隔离；对象存储 bucket/prefix、KMS/加密 key 的隔离级别。
- **[用户决策]** PG HA 方案、Redis Cluster/Sentinel、Temporal production topology、SeaweedFS topology、备份供应商与灾备地域。
- **[用户决策]** 迁移停机预算与数据驻留/合规要求；决定 online copy、双写/校验、cutover 和回退机制。

### 契约与事件

业务 OpenAPI 不暴露物理 schema / cluster；请求通过已认证 team 解析到 `TenantRoute`。仅平台运维 API 允许查询抽象的 tenant placement，响应不得含 DSN/证书/基础设施 IP。

内部事件至少包括：`tenant.provisioned.v1`、`tenant.migration.requested.v1`、`tenant.migration.phase-changed.v1`、`tenant.route.cutover.v1`、`tenant.migration.verified.v1`、`tenant.migration.rolled-back.v1`。每个事件必须 idempotent、可关联 migration job 和 request/trace；切换事件进 trusted 审计。

### 迁移与数据模型

- 控制面 registry（建议 `tnt_tenant_placement`，前缀已有 `tnt_` 但需要更新正式登记）：team、placement type、route version、目标 reference、state、cutover timestamp、审计字段。只存 Secret/connection reference，绝不存明文 DSN。
- 如 schema-per-tenant：迁移工具接受 tenant/schema 参数，以同一 migration manifest 应用，校验 checksum 与 shared RLS schema version；需明确 `gp.set_tenant()` 语义在独立 schema 中保留还是裁剪，且不能在未经审计的情况下关闭 RLS。
- 如 database/cluster-per-tenant：建立 provisioning 和 migration jobs，所有 repository 从 `TenantRouteResolver` 获得租户正确 pool；pool 生命周期、连接上限、故障降级需测试。
- 跨租户汇总/平台运维报表是未定义需求。默认禁止跨物理租户全表扫描；如业务需要，另立只读受控聚合管道与授权模型。

### 分层职责

- `platform/domain`：TenantRoute、Placement、迁移状态机与安全切换不变量；不引用 pgx/K8s。
- `platform/application`：解析 team → route、发起/推进迁移工作流、冻结写入/一致性校验/cutover/补偿；跨域调用统一经端口。
- `platform/infra`：连接池工厂、路由 registry repository、K8s/Cloud provisioner、备份接口、Redis/Temporal/对象存储 cluster clients。
- `gateway`：认证后提取 team，选择 route，注入请求上下文；不能允许客户端 header 指定 schema 或 endpoint。
- `cmd/worker`：长迁移/校验执行应为 Temporal workflow/activity 或受控 job；使用可重试/可观察步骤。

### 验收与依赖

- 共享库 RLS、schema-per-tenant 或独立库的同一组 CRUD/run/gate/audit 契约测试；任意错误路由应 fail closed。
- 迁移演练：源/目标 row count、hash/审计连续性、成本总览对账、对象存储 evidence 可读、Temporal run 不丢；切换和回退均有 trace/audit。
- HA 演练：PG 主切、Redis failover、Temporal worker 重连、对象存储故障；多副本 server/worker 保持正确性。
- 依赖：GP2-03/04/07/08 必须已落到 Redis/Temporal/持久资源事实实现；GP3-03/04 对时序和审计锚点的设计必须完成；GP4-04 提供 chart 与运行配置。可先开发 placement 模型、resolver 端口、迁移 dry-run/校验 CLI。

## 6. GP4-06 多团队高并发压测 + 监控告警

### 目标与范围

为多团队并发运行建立可重复压测、SLO、指标、看板、告警和处置手册，并以 GP2 的真实共享调度/资源实现为被测对象。没有用户认可的容量目标和环境基线时，只能交付压测框架、指标字典与场景，不应声称 QPS/时延达标。

### 前置决策与外部依赖

- **[用户决策]** N 团队、M 工程、每工程用例/资源类型、运行时长、读写比例、环境容量、目标 QPS、P95/P99、queue delay、RTO/RPO 和允许成本。
- **[用户决策]** 生产/预发压测窗口、SUT 隔离、是否允许生成真实证据/调用模型、数据清理策略；不得对 im-saas 或共享生产系统施压。
- **[用户决策]** Prometheus/Loki/Tempo/Grafana/Alertmanager 是否是标准栈，告警路由（IM/email/PagerDuty 等）、on-call owner 和升级策略。
- 依赖 GP2-01/03/04/07/08 的 Redis、Temporal、多 worker、资源池实际集成；否则仅能测 HTTP/Mock runner，不代表平台真实容量。

### 指标、事件与接口

不需要向普通用户新增业务 OpenAPI。可新增受 RBAC 保护的只读运维摘要接口，或直接以 Prometheus `/metrics` 暴露：

- HTTP：request count/latency/error，有限标签为 route/method/status_class，不带 team/user/run/case/target ID。
- 执行：run accepted/completed/failed/blocked、queue wait、dispatch latency、run duration、case result、quota deny、conflict deny、pool utilization。
- 依赖：Redis command error/latency、Temporal activity/workflow queue lag、PG pool/transaction error、对象存储失败、CI callback retry、认证失败率。
- 业务隔离：每个 team 仅用于日志/trace 的关联 ID；指标标签只允许有限 tenant tier 或 route class，避免高基数。

告警规则须逐条登记影响、阈值、持续时间、owner 和 runbook。初始候选（阈值均 **[用户决策]**）：持续错误率、readiness 不可用、队列积压、P95 queue wait、配额拒绝异常、资源池耗尽、Redis/PG/Temporal 不可用、审计 append/verify 失败、回写积压、认证异常率。

压测事件可写入独立 `ops_load_test_run` 投影（如需保留），但不能污染业务 audit/cost。若保留，字段包含场景版本、目标环境、开始/结束、生成负载摘要、结果与报告引用；表前缀须先登记。所有压测 run 使用显式 `run_mode=loadtest` 或隔离 team，具体行为需不影响生产门禁/回写。

### 实现职责与测试包

- `internal/platform/observability`：OTel meter/tracer 注册、有限 labels、敏感字段过滤；不得把业务规则写入 metric callback。
- `execution/application/infra`：在准入、排队、领取、dispatch、终态处发射指标和 trace；所有采集失败不可阻断真实 run。
- `deploy/monitoring/`：PrometheusRule、Dashboard as code、Alertmanager routing templates、runbooks；部署与 Helm values 联动。
- `test/load/`：k6/Locust/Gatling（工具待定）脚本、数据 seed、预检、清理、结果阈值；负载脚本和断言进版本库。

### 验收与依赖

- 指标测试：关键 HTTP/run/queue/worker 路径产生预期 counter/histogram/trace；没有 team/user/run 等高基数标签；敏感信息不出现。
- 告警演练：人为注入一个依赖失败/队列积压/审计错误，验证在约定窗口内触发、携带 runbook、恢复后可解析；不以截图代替演练记录。
- 容量测试：多团队中一个 team 用满自身配额，其他 team 的 admission/queue wait 满足阈值；横向增加 server/worker 后吞吐与目标曲线相符；停止一个 worker/调度器/执行节点任务可被接管。
- 可复现：同一版本的 load profile、基础设施 values、结果报告、Grafana 链接/导出和假设均存档；与生产混跑须有明确批准。

## 7. 推荐开发顺序与并行边界

### 建议串行主线

1. **先定横向决策与 ADR**：身份源、CI 首批平台、私有化拓扑、租户分区、SLO。没有这些，GP4-02～06 的外部边界无法正确实现。
2. **GP4-03 的 Principal/认证基础 + GP4-01 的授权模型**：可并行开发不依赖 provider 的 domain/application/迁移；认证中间件接入后完成真实强制授权。不要先将 UI 按钮隐藏当作 RBAC 完成。
3. **GP2 真实共享调度基础完成后接 GP4-02**：先做 provider-independent connector/outbox/CLI 契约和 fake；再选定一个 CI provider 实现端到端闭环。
4. **GP3 审计锚定与 GP4-04 打包一起完成**：独立 audit verifier 需要锚点定义、最小权限读取和可交付部署形态。
5. **GP4-05**：在真实 Redis/Temporal/资源池以及私有化 chart 存在之后做分区/集群化。先 dry-run/resolver/迁移校验，最后才能 cutover。
6. **GP4-06**：以稳定的生产候选架构做容量与故障验证，结果反哺 HPA、资源池、数据库/Redis/Temporal 参数和告警阈值。

### 可以立即开始的低风险编码包

- GP4-01：固定角色/asset permission 的领域模型、PostgreSQL 迁移、应用授权门、fake/httptest、OpenAPI 草案；先用测试 Principal port，保留真实 Auth middleware 接口。
- GP4-02：connector/delivery/outbox 领域与迁移、通用 idempotency、CLI 骨架、provider fixture 合同测试；真实 provider adapter 等用户选型。
- GP4-03：identity ports、Principal context、session 抽象、开发环境 fake provider 和安全测试；真实 WeChat/SMS/IdP adapter 等外部凭证和回调信息。
- GP4-04：Helm chart 目录与 values schema、build metadata、readiness/version、迁移 Job、安装/升级 runbook；不要假定依赖子 chart。
- GP4-05：TenantRoute resolver 端口、placement registry 迁移、dry-run/consistency checker；不执行真实客户数据迁移。
- GP4-06：OTel/Prometheus 指标骨架、load-test harness、dashboard/alert rule 模板；阈值均参数化等待 SLO。

### 阶段完成定义

GP4 不能以「代码能编译」或「页面显示」结案。至少需要：OpenAPI/前端类型一致、分层测试和真实依赖集成测试、RLS/授权越权测试、审计证据、安装或 provider sandbox 演练、以及针对 GP4-05/06 的故障/容量验证。每项对尚未具备的外部账号、集群、SLO 或客户数据迁移必须如实标为待验收。
