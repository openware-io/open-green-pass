# M3 clientRelease 高风险补齐（2026-10-25）

## 新增

为 12 个 clientRelease 上传相关 Cell 创建真实请求：

```text
upload-sessions: happy / mime / size / presign / cancel / checksum
complete:        happy / mime / size / presign / cancel / checksum
```

run `2910733689294848`：

```text
5 pass
7 fail
```

## 失败

```text
text/plain contentType 被接受
300 字符文件名被接受
platform=invalid 被接受
空 fileName/contentType 被接受
sizeBytes=-1 被接受
complete 使用错误 objectKey 返回 200
```

已创建 finding：

```text
2910734053478400 P2
制品上传会话未校验 contentType/platform/fileName
```

## clientRelease

```text
required=45
mapped=41
passed=29
design=91.1%
execution=91.1%
```

达到风险门禁 90%。

## 当前 M3

```text
required = 417
mapped   = 339
passed   = 245
failed   = 74
blocked  = 8
planned  = 102
design_coverage = 75.5%
```

## 风险门禁

```text
design_covered=315
passed=245
gate_passed=false
open_p0=1
open_p1=4
```

剩余阻断：

```text
openPlatform 78.6%（POST 返回 405）
P0/P1 = 1/4
```

## 清理

- 已删除上传到 MinIO 的 15 字节测试对象
- 已清理敏感词/violation/config 测试数据
