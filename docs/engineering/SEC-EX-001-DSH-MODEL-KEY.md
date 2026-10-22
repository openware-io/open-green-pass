# 安全例外 SEC-EX-001：DSH 模型密钥环境变量注入

| 项目 | 值 |
|---|---|
| 状态 | **待安全负责人签署**；签署前仅允许本地/测试环境 |
| 日期 | 2026-10-22 |
| 关联 | ADR-001、Security Baseline §Secret references |

## 背景

GP 的模型凭据规范要求使用 `secret://`/`k8s://` 引用，由 AI 网关在调用时解析，不得以明文进入模型/审计/成本/运行记录。DSH SDK 运行时通过 `DEEPSEEK_API_KEY` 环境变量获取模型密钥，无法直接接受 GP 的 secret ref。

## 例外内容

- `gp-dsh-gateway` 从 K8s Secret 读取模型 Token；
- 仅在启动 DSH 子进程时注入 `DEEPSEEK_API_KEY`；
- 子进程生命周期 = 单 job，job 结束即销毁；
- 不把 Token 写入 job 输入、事件、transcript、日志、证据；
- Gateway 自身不持久化 Token，仅在内存中短暂持有。

## 风险

- 同一容器内同 UID 进程可能读取环境变量；
- 子进程崩溃转储/core dump 可能包含环境；
- 如果 DSH 插件/工具能执行 `env`，可能回显；
- 宿主机调试/`kubectl exec` 可读取进程环境。

## 控制措施

1. 专用 namespace/ServiceAccount，禁止 exec 权限给业务角色；生产只允许平台 SRE break-glass。
2. 非 root、只读根、seccomp、drop ALL caps、no privilege escalation。
3. 关闭 core dump：`ulimit -c 0`；Pod `securityContext` 限制。
4. DSH 工具白名单禁止 shell/env 读取；只读文件工具。
5. 子进程启动后，父进程不记录环境；stderr/stdout 脱敏。
6. 密钥短 TTL/可轮换；泄露可立即撤销。
7. 审计 `dsh_job_started`（只记 job_id/team/model route，不记 token）。
8. 生产前做进程环境读取测试/渗透检查。

## 审批

- 安全负责人：待签署
- 架构负责人：ADR-001 已接受
- 失效/复审：DSH 升级、密钥轮换机制变化、或 90 天后复审

## 未签署前的约束

- 仅本地/测试集群可用；
- 不接入生产租户数据；
- 发布记录标注 `SEC-EX-001 pending`。

## 签署后

- 允许生产 `gp-dsh-gateway` 使用该注入方式；
- 仍需满足上述控制与审计；
- 安全基线增加例外条目，指向本文档。
