# 生成审核入库失败修复（2026-10-23）

## 现象

批次 `2905666997489664` 在“人工审核 → 确认生效”后失败：

```text
审核入库失败：请检查候选质量复核、审核记录和版本化用例入库结果。
```

批次中 5 条候选质量通过，1 条质量未通过。

## 根因

1. **质量检查在源码内容清理后重跑**
   - `QualityGate` 首次通过后会 `PurgeSourceContents`，把 `gen_source_snapshot.source_content` 清空，只保留 normalized manifest。
   - 审批时 `CommitVersions` 再次调用 `QualityCheckSeeds`。
   - 旧逻辑只把 `files`（已清空）加入 `allowedSources`，没有使用 `normalized.Files`，导致所有候选都被判定“引用源码不在冻结快照中”，全部标记失败。
2. **单条候选失败会阻断整批**
   - `QualityCheckSeeds` 遇到无效候选直接返回错误，`CommitVersions` 失败。
   - `CommitSeeds` 又要求 `total == passed`，即使部分候选有效也不能入库。

## 修复

- `QualityCheckSeeds` 的 `allowedSources` 同时使用 `normalized.Files`，审批时重跑不再误判。
- 单条候选质量失败只标记该 seed 为 `failed`，不阻断整批。
- `CommitSeeds` 允许只提交 `quality_state='passed'` 的候选：
  - 失败候选标记 `approval_state='rejected'`；
  - 至少一条通过即可提交；
  - 重复提交幂等判断改为 `committedSeeds == passedSeeds`。

## 验证

新建批次 `2905722991808512`（pc-admin @ Tag 2.2.1）：

- 生成 6 条候选
- 审批后 quality 重跑通过
- 最终状态：`approved`
- 6 条 seed 全部 `quality_state=passed`、`approval_state=approved`
- 已写入用例库

说明审批时质量重跑不再因为源码清理而全量失败。

## 说明

- 原失败批次 `2905666997489664` 已进入 failed 终态，不能原地恢复；需要重新创建批次。
- 新批次仍使用真实 DSH 生成。
- 本次修复只涉及 GP 生成入库逻辑，不修改 im-saas 源码。
