#!/usr/bin/env bash
# push 门禁：status / gofmt / vet / test 全绿才许 push（尤其 force-with-lease）。
# 用法：
#   scripts/sync-gate.sh            # quick：门禁四项（约 1 分钟）
#   scripts/sync-gate.sh --full     # 全量：quick + 三构建 + 全测试包 + 前端测试
# 任意一项红即非零退出，直接停，不进 push/docs 步骤。
# 约定：测试一律 -count=1（禁缓存，前车之鉴：缓存假绿）。
set -euo pipefail
cd "$(dirname "$0")/.."

GO=/usr/local/go/bin/go
MODE="${1:-quick}"
# 低内存容器：重任务用 cgroup 硬限制包裹（GOMEMLIMIT 等 env 不是防线）
if command -v systemd-run >/dev/null 2>&1; then
  RUN=(systemd-run --user --scope -p MemoryMax=1G --)
else
  RUN=()
fi

pass() { echo "[gate:PASS] $1"; }
fail() { echo "[gate:FAIL] $1" >&2; exit 1; }

# 1. 工作区干净（未提交就推 = 丢修复；只拦 unstaged/untracked，已 add 待提交不算脏）
DIRTY="$(git diff --name-only; git ls-files --others --exclude-standard)"
if [ -n "$DIRTY" ]; then echo "$DIRTY" >&2; fail "工作区不干净，先 add/commit"; fi
pass "工作区干净"
# 2. gofmt（只查本次改动中的 Go 文件；基线预存漂移（Go 新版 gofmt 规则）不管，
#    否则门禁永红。改动 = staged + unstaged 的 .go 文件）
CHANGED_GO="$(git diff --name-only HEAD -- '*.go'; git diff --cached --name-only -- '*.go')"
CHANGED_GO="$(printf '%s\n' $CHANGED_GO | sort -u | grep '\.go$' || true)"
FMT_OUT=""
if [ -n "$CHANGED_GO" ]; then
  # shellcheck disable=SC2086
  FMT_OUT="$(gofmt -l $CHANGED_GO 2>/dev/null || true)"
fi
if [ -n "$FMT_OUT" ]; then echo "$FMT_OUT" >&2; fail "gofmt 脏，先 gofmt -w"; fi
pass "gofmt 干净"
# 3. vet（碰撞高发区；全绿才继续；单次判定，输出保留）
if ! VET_OUT="$("$GO" vet ./controller/ ./relay/helper/ ./relay/common/ 2>&1)"; then echo "$VET_OUT" >&2; fail "go vet 红，见上"; fi
pass "go vet 通过"

# 4. locale JSON 合法（防字节追加写坏）
python3 -c "
import json
for f in ['en.json','fr.json','ja.json','ru.json','vi.json','zh-TW.json','zh.json']:
    json.load(open(f'web/src/i18n/locales/{f}'))
" || fail "locale JSON 非法"
pass "locale JSON 合法"

# 5. 测试（quick：controller 全量约 1s；full：工作流三件套口径）
if [ "$MODE" = "--full" ]; then
  "${RUN[@]}" "$GO" build ./... || fail "根模块构建红"
  pass "根模块构建通过"
  (cd relaykit && GOWORK=off "$GO" build ./...) || fail "relaykit 构建红"
  pass "relaykit 构建通过"
  (cd web && bun run build >/dev/null) || fail "前端构建红"
  pass "前端构建通过"
  "${RUN[@]}" "$GO" test -count=1 ./controller/... ./service/... ./relay/... ./common/... ./pkg/billingexpr/... \
    || fail "Go 全测试包红"
  pass "Go 全测试包通过（-count=1）"
  (cd web && "${RUN[@]}" bun run test) || fail "前端测试红"
  pass "前端测试通过"
else
  "${RUN[@]}" "$GO" test -count=1 ./controller/... || fail "controller 测试红"
  pass "controller 测试通过（-count=1）"
fi

echo "门禁全绿，可以 push"
