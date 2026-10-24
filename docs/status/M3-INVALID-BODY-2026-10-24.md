# M3 非法请求体维度覆盖（2026-10-24）

## 范围

对 19 个未映射 Cell 创建空请求体用例：

```text
type_format
enum
partial_invalid
```

方法：

```text
POST/PUT
body={}
path {id} -> 999999
expected 400
```

## 执行

run `2909364156141568`：

```text
14 pass
5 fail
```

失败主要是接口返回其他错误码或未校验；保留失败，不改写。

## 当前 M3

```text
required = 417
mapped   = 301
passed   = 238
failed   = 55
blocked  = 8
planned  = 140
design_coverage = 66.4%
execution_coverage = 66.4%
```

## 说明

空请求体只能验证“缺少合法字段时拒绝”，不等价于完整的 type/enum 测试。  
后续仍需每个端点的参数 schema 才能精确覆盖类型与枚举边界。
