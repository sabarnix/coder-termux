#!/data/data/com.termux/files/usr/bin/bash
# One-shot bootstrap for native-Termux coder.
#
# Public one-liner:
#   curl -fsSL https://raw.githubusercontent.com/sabarnix/coder-termux/main/install.sh | bash
#
# What it does:
#   1. verifies you are on Termux (Android arm64)
#   2. installs the openssh + curl + jq packages if missing
#   3. downloads the pinned PIE arm64 coder binary from the GitHub release,
#      verifies its sha256, and installs it to $PREFIX/bin/coder
#   4. drops the mosh-coder wrapper into $PREFIX/bin
#   5. runs `coder login <url>` if you have not logged in already
#   6. runs `coder config-ssh --yes --coder-binary-path "$PREFIX/bin/coder"`
#      so `ssh <workspace>.coder` just works
#
# Env knobs:
#   REPO                override "sabarnix/coder-termux"
#   RELEASE_TAG         pin a release (default: "latest")
#   CODER_URL           skip the login prompt and use this deployment URL
#   SKIP_LOGIN=1        skip `coder login` entirely (assume ~/.config/coderv2 exists)
#   SKIP_CONFIG_SSH=1   skip `coder config-ssh`
set -eu

REPO="${REPO:-sabarnix/coder-termux}"
RELEASE_TAG="${RELEASE_TAG:-latest}"
ASSET_NAME="coder-linux-arm64"
SHA_NAME="${ASSET_NAME}.sha256"

say() { printf '==> %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

[ "$(uname -o 2>/dev/null || true)" = "Android" ] \
  || die "this installer is for Termux on Android — run inside Termux"
[ "$(uname -m)" = "aarch64" ] \
  || die "this installer is for arm64 Termux — your arch is $(uname -m)"
[ -n "${PREFIX:-}" ] && [ -d "$PREFIX/bin" ] \
  || die "\$PREFIX is unset or missing — are you really in Termux?"

say "installing termux packages (openssh curl jq)"
need_pkg=0
for p in openssh curl jq; do
  command -v "$p" >/dev/null 2>&1 || need_pkg=1
done
if [ "$need_pkg" = 1 ]; then
  pkg install -y openssh curl jq >/dev/null
fi

say "resolving release $RELEASE_TAG from $REPO"
if [ "$RELEASE_TAG" = "latest" ]; then
  manifest_url="https://api.github.com/repos/$REPO/releases/latest"
else
  manifest_url="https://api.github.com/repos/$REPO/releases/tags/$RELEASE_TAG"
fi
manifest="$(curl -fsSL "$manifest_url")" \
  || die "could not fetch release manifest from $manifest_url"
asset_url="$(printf '%s' "$manifest" | jq -r --arg n "$ASSET_NAME" '.assets[] | select(.name==$n) | .browser_download_url')"
sha_url="$(printf '%s' "$manifest" | jq -r --arg n "$SHA_NAME" '.assets[] | select(.name==$n) | .browser_download_url')"
[ -n "$asset_url" ] && [ "$asset_url" != "null" ] \
  || die "release $RELEASE_TAG has no asset named $ASSET_NAME"
[ -n "$sha_url" ] && [ "$sha_url" != "null" ] \
  || die "release $RELEASE_TAG has no asset named $SHA_NAME"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

say "downloading $ASSET_NAME"
curl -fsSL -o "$tmp/coder" "$asset_url"
curl -fsSL -o "$tmp/coder.sha256" "$sha_url"

expected="$(awk '{print $1}' "$tmp/coder.sha256")"
actual="$(sha256sum "$tmp/coder" | awk '{print $1}')"
[ "$expected" = "$actual" ] \
  || die "sha256 mismatch for $ASSET_NAME: expected $expected, got $actual"
say "sha256 ok ($expected)"

install -m 755 "$tmp/coder" "$PREFIX/bin/coder"

say "installing mosh-coder wrapper"
curl -fsSL -o "$tmp/mosh-coder" "https://raw.githubusercontent.com/$REPO/main/client/mosh-coder"
install -m 755 "$tmp/mosh-coder" "$PREFIX/bin/mosh-coder"

if [ "${SKIP_LOGIN:-0}" != 1 ]; then
  if [ -f "$HOME/.config/coderv2/session" ] && [ -s "$HOME/.config/coderv2/url" ]; then
    say "coder session already present at ~/.config/coderv2 (skipping login)"
  else
    url="${CODER_URL:-}"
    if [ -z "$url" ] && [ -t 0 ]; then
      printf 'Coder deployment URL (e.g. https://coder.example.com): '
      read -r url
    fi
    [ -n "$url" ] || die "no CODER_URL provided — rerun with CODER_URL=... or SKIP_LOGIN=1"
    say "running: coder login $url"
    coder login "$url"
  fi
fi

if [ "${SKIP_CONFIG_SSH:-0}" != 1 ]; then
  say "writing ~/.ssh/config entries (coder config-ssh)"
  coder config-ssh --yes --coder-binary-path "$PREFIX/bin/coder"
fi

cat <<EOF

==> done.

Try:
    coder list                # see your workspaces
    ssh <workspace>.coder     # plain ssh
    mosh-coder <workspace>    # mosh over coder --udp (optional)

For a persistent terminal session, see:
    https://github.com/$REPO/blob/main/docs/herdr.md

EOF
