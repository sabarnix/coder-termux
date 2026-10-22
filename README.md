# coder-termux

Run [Coder](https://coder.com/) workspaces from Termux on Android — natively.

Google-Play-build Termux can't load most pre-built Linux arm64 binaries, and
the upstream `coder` CLI hangs inside `proot-distro` because of how Android
restricts netlink. This repo contains:

- A **source patch set** for `coder` v2.34.0 that produces a Termux-compatible
  PIE build with an Android-safe network-interface enumerator.
- A **build script** that applies the patch and emits a staged binary.
- **Installer scripts** that drop the staged binary into `$PREFIX/bin` and
  write `~/.ssh/config` so `ssh <workspace>.coder` works.
- A **client-side mosh wrapper** (`mosh-coder`) that tunnels mosh UDP through
  `coder port-forward --udp`, so you can hold a session across network changes.
- Notes on using [herdr](https://herdr.dev) as a persistent terminal workspace
  manager on the remote.

## Why

Three things conspire against running `coder` on the stock Play Store Termux:

1. **PIE requirement.** Android 10+ Bionic refuses non-PIE ELFs
   (`unexpected e_type: 2`), and the official `coder` release binary is non-PIE.
2. **TLS alignment.** Bionic's arm64 loader requires `PT_TLS` program headers
   with `p_align >= 64`. Go's non-cgo arm64 toolchain emits `p_align = 8`.
3. **Netlink is denied.** Android's `unpriv_sock_ioctls` SELinux policy
   returns `EPERM` for `RTM_GETLINK` and `RTM_GETADDR`, which breaks
   `net.Interfaces()` and tailnet startup. Running `coder` inside
   `proot-distro` instead swaps the problem: netlink *replies* queue but
   `epoll` never fires, so Go's `net.Interfaces()` hangs forever.

The patch set addresses all three.

## Install

One-liner, from a fresh Termux shell:

```sh
curl -fsSL https://raw.githubusercontent.com/sabarnix/coder-termux/main/install.sh | bash
```

The script installs `openssh`/`curl`/`jq`, downloads the pinned PIE arm64
`coder` binary from the GitHub release (sha256-verified), drops the
`mosh-coder` wrapper into `$PREFIX/bin`, runs `coder login` to collect your
deployment URL and session, and writes `~/.ssh/config` with
`coder config-ssh --yes --coder-binary-path "$PREFIX/bin/coder"`.

You can preseed the URL or skip steps with env vars:

```sh
CODER_URL=https://your-deployment \
  curl -fsSL https://raw.githubusercontent.com/sabarnix/coder-termux/main/install.sh | bash
```

See `install.sh` for all knobs (`REPO`, `RELEASE_TAG`, `SKIP_LOGIN`,
`SKIP_CONFIG_SSH`). For a reinstall after rebuilding, use
`install/reinstall.sh`.

## Usage

### Plain SSH

```sh
ssh <workspace>.coder
```

That's it — `coder config-ssh` wrote a `ProxyCommand` that uses the native
Termux `coder` binary.

### Mosh over coder's UDP forward

```sh
mosh-coder <workspace>
```

The wrapper SSHes into the workspace, launches `mosh-server new -s -p <port>`,
starts `coder port-forward --udp <port>:<port>` in the background, and
executes `mosh-client 127.0.0.1 <port>` with the server's key. Caveat: Coder's
UDP forward has "TCP-like semantics" and in loopback tests dropped roughly
4/5 packets on a cold connection; mosh's own retransmit absorbs this but
the first second or two can feel sticky.

### Herdr persistent workspace

With herdr already installed on the remote (see `docs/herdr.md`):

```sh
ssh -t <workspace>.coder /home/<user>/.local/bin/herdr
```

Herdr handles session persistence on the server, so a dropped ssh just means
re-running the command to reattach.

## Build it yourself

See `patches/README.md` and `patches/build.sh`.

## Layout

```
install/   first-time and reinstall scripts
client/    the mosh-coder wrapper and a profile.d snippet with aliases
patches/   go sources that need to land in coder v2.34.0 + the build script
docs/      herdr, mosh, troubleshooting notes
```

## License

MIT. See `LICENSE`.
