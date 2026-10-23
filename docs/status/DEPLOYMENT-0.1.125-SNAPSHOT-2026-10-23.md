# GreenPass kind 发布记录：0.1.125-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.125-SNAPSHOT` |
| 部署 checkout | `e12f587` |
| schema | 43 |
| server / worker | `sha256:57214285742684ba1c58c87614bbbfeeaf2763e58182eb086d5a58926d242f5c` |
| web | `sha256:7e7d6bec0273bf76b56d1f5141fb0d59b9f3d1ac77146ccebbc4523ac7268d43` |
| dsh-gateway | `sha256:b8137194db099121f592a09e21392a59bce5ed08d08f84f699ecb49c67021104` |

## 新增

- 真实 DSH SDK gateway 镜像（Node 22 + dsh 0.2.1-rc.2）
- 生成批处理走 DSH job
- 生成页显示 DSH 生成，不再显示项目模型绑定

## 验收

| 检查 | 结果 |
|---|---|
| gateway rollout | 成功 |
| DSH smoke job `2905358401474560` | succeeded |
| 真实 batch `2905359519387648` | review |
| seed 数 | 7 |
| seed provider | `dsh / dsh-sdk` |
| 生成页 | DSH banner |
| 真实执行 run | pass（此前 0.1.122） |
| IM 隔离 | 通过 |

## 说明

- 第一次 0.1.125 因 secret key 写错为 `api-key` 导致 `CreateContainerConfigError`，改为 `token` 后重试成功。
- gateway 镜像较大（Node + DSH），首次构建约 4 分钟；后续有层缓存。
