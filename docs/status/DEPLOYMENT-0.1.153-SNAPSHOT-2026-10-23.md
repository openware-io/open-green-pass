# GreenPass kind 发布记录：0.1.153-SNAPSHOT（2026-10-23）

| 项目 | 值 |
|---|---|
| 版本 | `0.1.153-SNAPSHOT` |
| schema | 48 clean |
| 变更 | Findings 证据上传按钮接入真实执行接口 |
| 验证 | UI bundle 上传文案存在；3MiB 上传 201；下载 200 |

## 说明

- Findings 页“补充证据”从原型改为文件选择。
- 只对真实数字 finding ID 启用；原型 finding 仍提示不支持。
- 上传走 GP 代理，不依赖集群内预签名 URL。
- 上传后刷新 finding 详情。
