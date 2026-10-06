#!/usr/bin/env bash
# GreenPass 迁移校验（跨平台主脚本；Windows 原生等价 validate-migrations.ps1）
# 检查：文件名格式 / up-down 配对 / 表前缀归属（对照 ENGINEERING-SPEC §8 登记表）
set -euo pipefail
MIG_DIR="${1:-migrations}"
# ENGINEERING-SPEC §8 登记前缀
prefixes="tgt_ cas_ ctr_ rpt_ tnt_ iam_ mdl_ repo_ run_ res_ env_ gen_ gate_ cost_ aud_ evd_ ts_ svc_"
err=0
declare -A up down

# 1) 文件名格式 + 归类
for f in "$MIG_DIR"/*.sql; do
  [ -e "$f" ] || continue
  base=$(basename "$f")
  if [[ ! "$base" =~ ^[0-9]{6}_.+\.(up|down)\.sql$ ]]; then
    echo "migcheck: 非法迁移文件名: $base (期望 000NNN_name.up.sql / .down.sql)"; err=1; continue
  fi
  num=${base%%_*}
  kind=${base##*.}
  if [ "$kind" = "up" ]; then up[$num]=1; else down[$num]=1; fi
done

# 2) up/down 一一对应
for num in "${!up[@]}"; do
  [ "${down[$num]:-}" = "1" ] || { echo "migcheck: 迁移 $num 缺少 .down.sql"; err=1; }
done
for num in "${!down[@]}"; do
  [ "${up[$num]:-}" = "1" ] || { echo "migcheck: 迁移 $num 缺少 .up.sql"; err=1; }
done

# 3) 表前缀归属
for f in "$MIG_DIR"/*.up.sql; do
  [ -e "$f" ] || continue
  while read -r line; do
    name=$(echo "$line" | sed -E 's/^[[:space:]]*CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_\.]+).*/\2/')
    [ -z "$name" ] && continue
    base=${name##*.}
    ok=0
    for p in $prefixes; do
      case "$base" in "$p"*) ok=1;; esac
    done
    [ $ok -eq 1 ] || { echo "migcheck: $f 中表 $base 前缀未登记（ENGINEERING-SPEC §8）"; err=1; }
  done < <(grep -iE '^[[:space:]]*CREATE TABLE' "$f" || true)
done

[ $err -eq 0 ] && echo "migcheck: OK" || exit 1
