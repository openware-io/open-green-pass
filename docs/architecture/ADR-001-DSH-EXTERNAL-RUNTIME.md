# ADR-001：DSH 作为外部智能体运行时，不进入 GreenPass 业务域

| 项目 | 值 |
|---|---|
| 状态 | Accepted（架构方向）；安全例外 SEC-EX-001 待安全负责人签署 |
| 日期 | 2026-10-22 |
| 决策人 | GreenPass 架构/产品（用户批准“全部按建议”） |
| 关联 | `DSH-SDK-GATEWAY-SPEC.md`、`ENGINEERING-SPEC.md` §2/§12 |

## 背景

GreenPass 后端是 Go 单体 + worker；DSH 是 Node/TypeScript 智能体运行时，提供 SDK（JSON-RPC stdio）、headless、ACP。测试引擎需要 DSH 的规划、生成、分析能力，但不能把 DSH 的业务状态、权限和工具直接暴露给 GP 业务域。

## 决策

1. 新增部署单元 `cmd/dsh-gateway`，用 Go 实现：
   - HTTP/内部 API、队列、worker 池、job 状态机、超时/取消、事件采集；
   - 不拥有 GP 业务数据，不直连 `gp_` 数据库；
   - 通过 GP API/事件回写 job 状态和结果。
2. DSH 以**外部 Node 子进程**运行：
   - 固定 `@deepseek-ai/dsh` 版本；
   - 只加载 `gp-test-engine` profile；
   - 每 job 独立进程/workspace/`DSH_HOME`；
   - 工具只读；禁止写源码、git 写、部署、Web。
3. 镜像：`ghcr.io/openware-io/open-green-pass-dsh-gateway:X.Y.Z-SNAPSHOT`，Go 静态二进制 + 固定 Node/DSH 运行时，非 root、只读根、SBOM/漏洞扫描。
4. 部署：只内网 Service，不暴露 ingress；纳入 `init-kind.ps1`/`deploy-kind.ps1`/`deploy/k8s/`/CI；readiness/liveness；资源配额。
5. 数据边界：
   - `gen_job`/`gen_job_event` 归 AI 域，由 GP 持久化；
   - `def_*` 归 Governance；
   - `evd_evidence` 归 Trusted，gateway 不直接写。
6. 安全基线：
   - 专用 SA、`automountServiceAccountToken: false`、非 root、RuntimeDefault seccomp、drop ALL caps、allowPrivilegeEscalation false、只读根；
   - egress 先 staging default-deny + allowlist 验证；
   - `DEEPSEEK_API_KEY` 通过 Secret 注入 DSH 子进程（例外 SEC-EX-001，禁止日志/回显）。

## 不采用的替代方案

| 方案 | 不采用原因 |
|---|---|
| 把 `dsh web` 当后端 | 交互式 UI、无多租户/队列/审计，工具权限过宽 |
| 在 Go 里重写 DSH | 重复实现 Skills/MCP/工具运行时，维护成本高 |
| 让 DSH 直连 GP DB | 违反数据所有权/RLS/可信域边界 |
| 一进程多租户共享 session | 工具/文件/上下文隔离不足，崩溃影响面大 |

## 后果

- 新增一个部署单元和一个第三方 Node 运行时，需要独立的构建、扫描、发布和监控。
- GP 与 DSH 通过内部 API/事件解耦；DSH 不可用时可降级到 headless/禁用生成，不影响平台主链路。
- 需要维护 `gp-test-engine` profile 与 DSH 版本升级流程；升级必须走发布评审。
- 安全例外 SEC-EX-001 必须在生产前由安全负责人签署；否则只能用于本地/测试环境。

## 验收

- gateway 无 `gp_` DB 连接串；无源码写路径。
- job 取消/超时后进程组消失、workspace 删除。
- 网关重启可恢复/清理 job。
- 镜像 SBOM/漏洞扫描通过；非 root/只读根/seccomp 检查通过。
- egress staging 验证通过后才允许生产 default-deny。
