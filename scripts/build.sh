#!/usr/bin/env bash
# build.sh
#
# Build the tmux-cc-monitor binary for the TPM plugin install path.
# Called from the TPM entry point script (tmux-cc-monitor.tmux) on each
# tmux startup; rebuilds only when bin/tmux-cc-monitor is missing or older
# than any source file under cmd/, internal/, go.mod, or go.sum.
#
# Exit codes:
#   0  binary is already up-to-date, or build succeeded
#   1  'go build' failed
#   2  'go' command not found in PATH (and a build was actually needed)
#
# Surfaces error context on stderr; the caller decides how to display it
# (tmux-cc-monitor.tmux turns non-zero exits into `tmux display-message`).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN_DIR="${REPO_ROOT}/bin"
BIN="${BIN_DIR}/tmux-cc-monitor"

needs_build=0
if [[ ! -x "${BIN}" ]]; then
  needs_build=1
else
  # `find ... -newer "${BIN}" -print -quit` stops at the first newer file
  # so we don't walk the whole tree just to learn that one thing changed.
  newer="$(find \
    "${REPO_ROOT}/cmd" \
    "${REPO_ROOT}/internal" \
    "${REPO_ROOT}/go.mod" \
    "${REPO_ROOT}/go.sum" \
    -newer "${BIN}" -print -quit 2>/dev/null || true)"
  if [[ -n "${newer}" ]]; then
    needs_build=1
  fi
fi

if [[ "${needs_build}" -eq 0 ]]; then
  exit 0
fi

if ! command -v go >/dev/null 2>&1; then
  echo "tmux-cc-monitor: 'go' command not found in PATH" >&2
  exit 2
fi

mkdir -p "${BIN_DIR}"

cd "${REPO_ROOT}"
if ! go build -o "${BIN}" ./cmd/tmux-cc-monitor; then
  echo "tmux-cc-monitor: 'go build' failed" >&2
  exit 1
fi

exit 0
