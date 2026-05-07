#!/usr/bin/env bash
# tmux-cc-monitor.tmux
#
# TPM (Tmux Plugin Manager) entry point. Sourced/executed by tmux when the
# plugin is installed via `set -g @plugin 'ch0wdreN/tmux-cc-monitor'`.
#
# Responsibilities:
#   1. Build the tmux-cc-monitor binary on demand (delegated to scripts/build.sh).
#   2. Read user-configurable tmux options.
#   3. Bind the launch key to open the popup TUI.
#
# User-configurable options (set in tmux.conf with `set -g <name> '<value>'`):
#   @cc-monitor-key            launch key after the prefix (default: C-g)
#   @cc-monitor-popup-width    popup width passed to display-popup -w (default: 80%)
#   @cc-monitor-popup-height   popup height passed to display-popup -h (default: 80%)

PLUGIN_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="${PLUGIN_DIR}/bin/tmux-cc-monitor"

# Build (or skip-build if already up-to-date) before binding. Output stays on
# stdout/stderr so `prefix + I` shows it in TPM's install popup.
"${PLUGIN_DIR}/scripts/build.sh"
build_exit=$?

if [[ ${build_exit} -ne 0 ]]; then
  case ${build_exit} in
    2)
      tmux display-message "tmux-cc-monitor: 'go' not found. Install Go to use this plugin."
      ;;
    *)
      tmux display-message "tmux-cc-monitor: build failed. Run ${PLUGIN_DIR}/scripts/build.sh to see details."
      ;;
  esac
  # Exit 0 anyway: a non-zero return from a TPM plugin script is not useful,
  # and the user has already been notified via display-message.
  exit 0
fi

get_option() {
  local opt="$1"
  local default="$2"
  local value
  value="$(tmux show-option -gqv "${opt}")"
  if [[ -z "${value}" ]]; then
    echo "${default}"
  else
    echo "${value}"
  fi
}

key="$(get_option '@cc-monitor-key' 'C-g')"
popup_width="$(get_option '@cc-monitor-popup-width' '80%')"
popup_height="$(get_option '@cc-monitor-popup-height' '80%')"

tmux bind-key "${key}" display-popup -E -w "${popup_width}" -h "${popup_height}" "${BIN} ui"
