# M4 egress 未验证例外（2026-10-24）

## 现状

`deploy/k8s/gp-runner-policy.yaml` 已定义：

- ingress 仅允许 gp
- egress default deny
- allow DNS / gp / open-im-local gateway:3002

但实测：

```text
gp-server.gp.svc.cluster.local:8080/healthz -> 200（允许）
http://example.com -> 200（未阻断）
```

在 `gp-runner` 运行的 Node 测试确认 kind 默认 CNI 不执行 NetworkPolicy。

## 结论

当前 kind 环境：

```text
egress 隔离：未验证 / 未生效
```

不能宣称 egress policy 已通过。策略文件保留，作为支持 NetworkPolicy 的集群的部署基线。

## 继续验证所需

- 使用 Calico/Cilium 等支持 NetworkPolicy 的 CNI；或
- 在另一个支持 egress 策略的集群验证；或
- 由发布负责人正式接受“egress 未验证”的例外。

## 不允许

- 为了让检查通过而删除 NetworkPolicy
- 把 example.com 可达解释为“已隔离”
- 在未验证时宣称 egress 通过
