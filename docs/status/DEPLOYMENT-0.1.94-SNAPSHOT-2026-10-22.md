# GreenPass kind 发布记录：0.1.94-SNAPSHOT（2026-10-22）

## 发布基线

| 项目 | 值 |
|---|---|
| 环境 | kind `open-im-local`，命名空间 `gp` |
| 访问入口 | `http://greenpass.localhost:30080` |
| 部署版本 | `0.1.94-SNAPSHOT` |
| Git 分支 | `fix/scm-pat-auth-type` |
| 部署 checkout | `7aa0d21113afb6a3f7cda402df857e13268d3c66` |
| 部署入口 | `scripts/dev/init-kind.ps1 -Version 0.1.94-SNAPSHOT -ImageSource local-build` |
| 镜像来源 | 本地构建（`local-build`），未推送 GHCR |
| 部署时间 | 2026-10-22 17:00（Asia/Shanghai） |

## 镜像与制品

| 制品 | Tag | Image ID |
|---|---|---|
| server / worker | `ghcr.io/openware-io/open-green-pass:0.1.94-SNAPSHOT` | `sha256:42ade7d5f3779c83cbbd130d8dc26378798132de553a1b189744a8bf77dc0198` |
| web | `ghcr.io/openware-io/open-green-pass-web:0.1.94-SNAPSHOT` | `sha256:3b8de5e5127aa3f71edd423d2d55c4ec3e3c5770f66278088b87c1f82ce9bb99` |
| migrate | `ghcr.io/openware-io/open-green-pass-migrate:0.1.94-SNAPSHOT` | `sha256:db97f78aefe5472add1f2cfe6bd0fe05389ad813e60c33dcdbb269bbe55824ab` |

`gp-server` 2/2、`gp-worker` 2/2、`gp-web` 1/1，均为 `0.1.94-SNAPSHOT`。

## 本版原型补充

### 1. 缺陷归属仓库

缺陷中心和详情增加仓库信息：

- 仓库名：`open-chat-admin`
- 分支/提交：`2.2.1@a4487643`
- 源码位置：`src/api/<module>.js:<line>`
- 列表副行显示 `仓库@提交`；详情卡可跳转仓库 URL。
- 用途：缺陷能定位到具体仓库和冻结提交，而不是只到模块/端点。

### 2. 多媒体证据

详情新增“多媒体证据”区块：

- **截图证据**：必需；缩略图、点击查看大图、只读标识。
- **视频证据**：建议；视频封面、时长、说明、播放入口（原型）。
- 证据策略：
  - UI/实机用例必须提供截图；
  - 关键流程/复杂交互建议视频；
  - 服务端保存 MIME、尺寸、时长、SHA-256；
  - 证据只读、不可篡改；
  - 证据与 finding/run/case/commit 绑定。
- 列表“证据”列增加截图/视频标记；顶部统计显示截图证据 6 条、视频证据 1 条。
- 预留“补充证据”入口（原型提示，后续接预签名上传）。

### 3. 数据模型/API 方案同步

`docs/plans/GP-AI-TEST-ENGINE-EXECUTION-PLAN.md` 已补充：

- `finding` 增加 `repository_id/repo_url/branch/commit_sha`；
- `finding_evidence` 增加 `kind(screenshot/video)`、`mime_type`、`width/height`、`duration_ms`、`sha256`；
- 新增 `POST /findings/{id}/evidence:presign`、`POST /findings/{id}/evidence`、`GET /findings/{id}/evidence/{eid}`；
- 验收增加“P0/P1 有截图、UI/实机用例截图 100%”。

## 验收

| 检查 | 结果 |
|---|---|
| 列表仓库标识 | 6 条均显示 `open-chat-admin@a4487643` |
| 详情仓库卡 | `open-chat-admin` / `2.2.1@a4487643` |
| 截图证据 | 详情有截图缩略图，点击打开大图 |
| 视频证据 | F-1042 有视频封面/时长/说明/播放入口 |
| 证据策略 | 详情显示截图必需、视频建议、哈希/只读说明 |
| 初始列表 | 进入缺陷中心不自动打开抽屉 |

## 部署过程说明

首次 `0.1.94` 部署时，`open-im-local/saas-admin` 发生外部 rollout，脚本的 IM workload 隔离校验失败，按规范自动恢复 `0.1.93` 镜像并中止。等待 `saas-admin` generation 稳定后重新执行同一唯一入口，部署成功；未绕过隔离检查。

## 镜像清理

- Docker 本地清理 `0.1.93`，当前仅保留 `0.1.94`。
- 两个 kind 节点删除 `0.1.93`，当前仅保留 `0.1.94`。
- `0.3.x` 仍为空。

## 说明

本版仍是前端原型：缺陷/覆盖/截图/视频为原型数据，未写后端库、未改生成执行链路、未部署 DSH SDK 网关。评审后再进入 `finding`/`coverage_cell`/证据上传的真实实现。
