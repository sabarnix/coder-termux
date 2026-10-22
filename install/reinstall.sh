#!/data/data/com.termux/files/usr/bin/bash
# Reinstall the staged coder binary into $PREFIX/bin and reconnect.
# Expects a sibling file named "coder" (the PIE arm64 build).
#   bash reinstall.sh <workspace-name>
set -e
WS="${1:-}"
if [ -z "$WS" ]; then
  echo "usage: $0 <workspace-name>" >&2
  exit 2
fi
S="$(cd "$(dirname "$0")" && pwd)"
cp "$S/coder" "$PREFIX/bin/coder"
chmod 755 "$PREFIX/bin/coder"
echo "installed sha256 prefix: $(sha256sum "$PREFIX/bin/coder" | cut -c1-16)"
exec ssh "$WS.coder"
