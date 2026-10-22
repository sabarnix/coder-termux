#!/data/data/com.termux/files/usr/bin/bash
# First-time install of a native-Termux coder.
# Expects three files in the same directory as this script:
#   coder     — the PIE arm64 binary built from patches/ against coder v2.34.0
#   url       — a single line like "https://your.coder.deployment"
#   session   — the Coder session token (chmod 600; never commit)
# Usage, run in plain Termux (NOT inside proot-distro):
#   bash setup.sh <workspace-name>
set -e
WS="${1:-}"
if [ -z "$WS" ]; then
  echo "usage: $0 <workspace-name>" >&2
  exit 2
fi
S="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$HOME/.config/coderv2" "$PREFIX/bin"
cp "$S/coder" "$PREFIX/bin/coder"
chmod 755 "$PREFIX/bin/coder"
cp "$S/url" "$S/session" "$HOME/.config/coderv2/"
chmod 600 "$HOME/.config/coderv2/session"
command -v ssh >/dev/null || pkg install -y openssh
echo "--- coder version"
coder version
echo "--- ping $WS (short timeout; should finish within seconds)"
timeout 30 coder ping "$WS" || echo "ping failed/hung (exit $?)"
echo "--- writing ~/.ssh/config entries"
coder config-ssh --yes --coder-binary-path "$PREFIX/bin/coder"
echo "Done. Try:  ssh $WS.coder"
