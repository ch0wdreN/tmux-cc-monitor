#!/usr/bin/env bash
# check-no-capture-pane-in-state.sh
#
# Enforces the ADR-0003 invariant that the state-classification code path
# must never reference `tmux capture-pane`. The popup mirror feature
# (introduced in v0.1.0) intentionally uses capture-pane, but only inside
# the UI / tmuxutil layer. This script is the mechanical guard that keeps
# the two paths separated.
#
# See:
#   - docs/adr/20260506-use-claude-code-hooks-for-state.md  (ADR-0003)
#   - docs/design-doc/20260506_tmux_cc_monitor_popup_mirror_design.md
#       §5 Acceptance Criteria (last bullet) and §12 Decision 2.
#
# Behavior:
#   - Greps for the literal string `capture-pane` in *.go files
#     (including *_test.go) under the directories listed in TARGETS.
#   - Comments are intentionally NOT excluded: re-introducing the string
#     even in a comment is treated as a violation, to minimize the chance
#     of state-classification logic silently regressing.
#   - The Go-side mechanization of this same invariant
#     (internal/state/purity_test.go) necessarily contains the literal
#     string `capture-pane` (it is the needle being searched for), so it
#     is the one file deliberately exempted from this scan.
#   - Exits 1 with the offending lines printed if any match is found.
#   - Exits 0 with a short OK message otherwise.

set -euo pipefail

# Resolve repo root from this script's location so the script works
# regardless of the caller's cwd.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

TARGETS=(
  "internal/state"
  "internal/hook"
  "internal/cleanup"
  "internal/errlog"
)

NEEDLE='capture-pane'

# Build the list of existing target directories (skip ones that don't
# exist yet — packages may not all be present at every point in history).
existing_targets=()
for d in "${TARGETS[@]}"; do
  if [[ -d "${REPO_ROOT}/${d}" ]]; then
    existing_targets+=("${REPO_ROOT}/${d}")
  fi
done

if [[ ${#existing_targets[@]} -eq 0 ]]; then
  echo "OK: no state-classification directories present yet (nothing to check)"
  exit 0
fi

# `grep -r` returns non-zero when nothing is found. We do not want
# `set -e` to abort on that: capture into a variable, then judge.
#
# We exclude purity_test.go — that file is the Go-side mechanization of
# this very invariant and must contain the needle.
hits=""
hits="$(grep -rn --include='*.go' --exclude='purity_test.go' -F "${NEEDLE}" "${existing_targets[@]}" || true)"

if [[ -n "${hits}" ]]; then
  echo "FAIL: '${NEEDLE}' must not appear in state-classification paths (ADR-0003)." >&2
  echo "Offending lines:" >&2
  echo "${hits}" >&2
  exit 1
fi

echo "OK: capture-pane is not referenced in state-classification paths"
exit 0
