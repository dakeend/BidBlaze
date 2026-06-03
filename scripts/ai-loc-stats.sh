#!/usr/bin/env bash
# 统计各前端模块当前代码行数，供 docs/ai-log/ai-contribution-report.md 填表。
# 用法：
#   bash scripts/ai-loc-stats.sh              # 当前各模块行数
#   bash scripts/ai-loc-stats.sh <git-ref>    # 与某次提交相比的增删行数（git diff --stat）
set -euo pipefail
cd "$(dirname "$0")/.."

count() { # $1=label  $2..=paths
  local label="$1"; shift
  local total=0 f n
  while IFS= read -r f; do
    n=$(wc -l < "$f"); total=$((total + n))
  done < <(find "$@" -type f \( -name '*.ts' -o -name '*.tsx' \) 2>/dev/null \
            | grep -v 'openapi.d.ts' || true)
  printf '%-32s %6d 行\n' "$label" "$total"
}

if [ $# -ge 1 ]; then
  echo "== 相对 $1 的改动（git diff --stat）=="
  git diff --stat "$1" -- admin-web/src mobile-h5/src fixtures || true
  exit 0
fi

echo "== 当前代码行数（不含生成的 openapi.d.ts）=="
count "admin-web/src/lib"        admin-web/src/lib
count "admin-web/src/mocks"      admin-web/src/mocks
count "admin-web/src/components" admin-web/src/components
count "admin-web/src/pages"      admin-web/src/pages
count "admin-web (src 合计)"     admin-web/src
if [ -d mobile-h5/src ]; then count "mobile-h5/src" mobile-h5/src; fi
echo
echo "生成类型 openapi.d.ts:"
find . -name 'openapi.d.ts' -not -path '*/node_modules/*' -exec wc -l {} + 2>/dev/null || true
