# Troubleshooting

## `unexpected e_type: 2`

The binary isn't PIE. Rebuild with `-buildmode=pie` or re-stage the one from
`patches/build.sh`.

## `netlinkrib: permission denied`

`coder` is reaching Android netlink before the patched interface getter is
registered. Confirm:

- `cmd/coder/linker_exec_args.go` is in your `cmd/coder/` tree.
- Its `init()` runs: check `uname -a` reports `aarch64 Android` and
  `/proc/self/exe` points at `/system/bin/linker64` (or `linker`).
- `androidcompat/` is on the module path and compiled in.

## `coder ssh` hangs

If you launched inside `proot-distro`, that's expected: `net.Interfaces()`
hangs on the proot kernel-netlink round-trip. Use the native Termux build.

## `tail` hangs on pipe input inside proot

`tail` is uutils (Rust) in Ubuntu 26.04 and reopens `/dev/stdin`, which
`proot-distro` binds to the proot launcher's stdin. Use `head`, `sed -n`,
`awk`, or `apt install coreutils-from-gnu`.

## `os.Executable()` returns `/apex/.../linker64`

Expected under linker-exec. Pass `--coder-binary-path "$PREFIX/bin/coder"`
to `coder config-ssh`, as the installer does.

## No `/etc/resolv.conf` → DNS failures

The patch sets `net.DefaultResolver` with a dial function that reads
`$PREFIX/etc/resolv.conf` or falls back to `8.8.8.8`/`1.1.1.1`. If you see
Go DNS errors, confirm `linker_exec_args.go` ran at startup.

## HTTPS fails with x509 errors

`SSL_CERT_FILE` or `SSL_CERT_DIR` wasn't set. The patch picks up
`$PREFIX/etc/tls/cert.pem` or `/apex/com.android.conscrypt/cacerts`. If you
built without the patch, export one of these yourself.
