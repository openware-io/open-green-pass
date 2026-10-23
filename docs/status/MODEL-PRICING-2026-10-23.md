# 模型价目与拆分计费（2026-10-23）

## 目标

生成成本不再使用单一混合 `GP_GENERATION_UNIT_PRICE`，改为：

- 按模型配置价目；
- 输入、输出、缓存读分别计价；
- 价目有生效时间；
- 历史成本行保留当时的单价，不随后续调价变化。

## 数据

### cost_price（沿用 000010 表并扩展）

新增字段：

- `model_key`：DSH 实际模型名，例如 `dsh/deepseek-flash`
- `cache_read_per_1k`
- `created_by` / `updated_by`
- `updated_at`
- 唯一索引 `(team_id, model_key, effective_from)`

`model_id` 保留给旧模型目录，允许为空。

### cost_line_item

新增：

- `cache_read_tokens`
- `input_unit_price`
- `output_unit_price`
- `cache_read_unit_price`
- `price_id`
- `price_missing`

## 计算

```text
amount =
  tokens_in  / 1000 × input_per_1k
+ tokens_out / 1000 × output_per_1k
+ cache_read / 1000 × cache_read_per_1k
```

没有匹配价目时：

- 仍然写 tokens；
- `price_missing=true`；
- `amount=0`；
- 成本页显示告警。

## API

```text
GET  /cost/prices
POST /cost/prices
```

请求：

```json
{
  "model": "dsh/deepseek-flash",
  "input_unit_price": 0.002,
  "output_unit_price": 0.008,
  "cache_read_unit_price": 0,
  "effective_from": "2026-10-23T00:00:00Z"
}
```

单位：人民币 / 千 token。

## 实际模型

- DSH gateway `Result` 返回实际模型和 `cacheReadTokens`；
- `DSHJob` / HTTP 响应包含 `model`；
- 生成成本使用 `dsh/<model>`；\n- 缓存读 token 按独立单价计费；
- 不再固定写 `dsh/dsh-sdk`。

## 验证

价目：

```text
dsh/deepseek-flash
input 0.002 / 1K
output 0.008 / 1K
```

实测批次 `2906337428180992`：

```text
tokens_in  = 234
tokens_out = 3137
amount     = 0.025564
price_missing = false
```

run `2906337427951616`：

```text
tokens 234 / 3137
cost   0.025564
state  succeeded
```

## 运营配置

不需要再设置 `GP_GENERATION_UNIT_PRICE`。

在成本审计页配置模型价目，或调用 `POST /cost/prices`。

保留期仍由 `GP_GENERATION_RETENTION_HOURS` 控制。

## 迁移注意

`cost_price` 已存在于 000010，因此 000048 使用 `ALTER TABLE` 扩展，不重复建表。
本次部署中修复了历史 dirty=48 状态；当前 `schema_migrations` 为 48 clean。
