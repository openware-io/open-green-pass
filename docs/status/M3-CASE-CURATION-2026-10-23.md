# M3：user 用例数据绑定（2026-10-23）

## 已更新用例

通过 `POST /cases/{id}/versions` 创建新版本，使用真实测试数据：

| 用例 | 请求 | 期望 |
|---|---|---|
| user-gen-001 | GET `/api/v1/admin/users?sort=created_at&order=desc` | 200 + `items` |
| user-gen-002 | GET `/api/v1/admin/users?page=999&page_size=25` | 200 + `items` |
| user-gen-003 | GET `/api/v1/admin/users` + `Authorization: Bearer invalid` | 401 |
| user-gen-004 | GET `/api/v1/admin/users/12` | 200 |
| user-gen-005 | GET `/api/v1/admin/users/999999` | 404 |
| user-gen-007 | PUT `/api/v1/admin/users/12/status` 完整 DTO | 200 |
| user-gen-008 | PUT `/api/v1/admin/users/12/status` body `{}` | 400 |
| user-gen-009 | PUT `/api/v1/admin/users/12/status` expectedStatusVersion=999 | 409 |
| user-gen-011 | DELETE `/api/v1/admin/users/999999` body `{"confirmUsername":"missing-user"}` | 404 |

## 执行结果

run `2906718344613888`：

```text
9 pass
0 fail
```

通过：

```text
001 列表排序
002 分页空页
003 无效 token 401
004 用户详情 200
005 不存在用户 404
008 空 body 400
009 非法 status 400
011 缺 confirm 404
```

失败：

```text
无失败；007 使用完整 AdminUpdateUserStatusDto 后返回 200
```

## 结论

- user 读路径已有真实执行证据。
- 合法状态更新需要 `status/expectedStatusVersion/idempotencyKey/reason/correlationId` 完整 DTO；补齐后 200/409 均通过。
- M3 仍未关闭：3 条写操作用例、24 Cell 有效映射、18 模块。

## 缺测试数据：显式 blocked（2026-10-23）

run `2906734884589568`：

```text
9 pass
3 blocked
0 fail
```

- 006：需要非管理员 token 验证 403。
- 010：需要可删除的专用测试用户。
- 012：需要被引用的专用测试用户。

覆盖：18 passed / 6 blocked；未伪造通过。
