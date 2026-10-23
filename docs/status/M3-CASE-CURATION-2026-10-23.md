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
| user-gen-007 | PUT `/api/v1/admin/users/12/status` body `{"status":"active"}` | 200（实际 400） |
| user-gen-008 | PUT `/api/v1/admin/users/12/status` body `{}` | 400 |
| user-gen-009 | PUT `/api/v1/admin/users/12/status` body `{"status":"bogus"}` | 400 |
| user-gen-011 | DELETE `/api/v1/admin/users/999999` body `{"confirmUsername":"missing-user"}` | 404 |

## 执行结果

run `2906716355956736`：

```text
8 pass
1 fail
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
007 状态更新 PUT 返回 400 REQUEST_BODY_INVALID
```

## 结论

- user 读路径已有真实执行证据。
- 写路径的非法请求可验证（008/009/011），合法状态更新仍返回 400，不能伪造通过。
- M3 仍未关闭：3 条写操作用例、24 Cell 有效映射、18 模块。
