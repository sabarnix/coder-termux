# coder patches for Termux (Android arm64, Bionic linker-exec)

Three source changes against [coder/coder](https://github.com/coder/coder)
v2.34.0, plus a `build.sh` that applies them and produces a Termux-runnable
PIE binary.

## What the patches do

### `cmd/coder/linker_exec_args.go`

Dropped into `cmd/coder/`. In its `init()`:

- When `/proc/self/exe` resolves to `linker64` (Bionic's `linker-exec`
  invocation path on Android 10+), it drops `os.Args[0]`. The kernel places
  the real executable path in `argv[1]` for linker-exec; Bionic's libc hides
  it from the C side, but Go reads argv straight from the stack. `argv[0]`
  can't be used as a signal because `termux-exec` preserves the caller's
  `argv[0]`.
- In the same guard it calls
  `tailscale.com/net/interfaces.RegisterInterfaceGetter(androidcompat.Interfaces)`
  so the tailnet startup uses our Android-safe enumerator instead of
  `net.Interfaces()`, which would otherwise fail or hang.
- Fills in a `net.DefaultResolver` dial function pointing at the Termux
  `$PREFIX/etc/resolv.conf` nameservers (or `8.8.8.8`/`1.1.1.1`), because
  Android has no `/etc/resolv.conf`.
- Sets `SSL_CERT_FILE` to `$PREFIX/etc/tls/cert.pem` or `SSL_CERT_DIR` to
  `/apex/com.android.conscrypt/cacerts` when no Linux CA bundle is present.
  A `GOOS=linux` build never looks at Android's cert store on its own.

### `androidcompat/interfaces.go` and `..._internal_test.go`

New package. Exports an `Interfaces()` function that returns the same shape
as `net.Interfaces()`, implemented with:

1. `net.Interfaces()` — the normal path, if the OS allows it.
2. `RTM_GETADDR` via a netlink route socket — fallback for Android, which
   denies `RTM_GETLINK` but historically allowed `RTM_GETADDR`.
3. `SIOCGIFCONF` + `SIOCGIFINDEX`/`FLAGS`/`MTU`/`NETMASK` ioctls — final
   fallback for Termux builds (observed on Pixel 8 / Android 17) where both
   netlink requests return `EPERM` under
   `unpriv_sock_ioctls`. These ioctls are permitted. IPv4 only.

The internal test cross-checks (2) and (3) against `net.Interfaces()` when
the current environment allows it, so you can run it inside proot-distro or
on the device itself.

### `build.sh`

Runs inside Ubuntu under `proot-distro` (the Go toolchain used here is itself
non-PIE and only loads under proot):

1. Builds `./cmd/coder` with `-buildmode=pie -tags
   slim,ts_omit_aws,ts_omit_bird,ts_omit_tap,ts_omit_kube` and a trimmed
   ldflags set.
2. Patches the `PT_TLS` program header's `p_align` from 8 to 64. Bionic
   arm64 refuses TLS alignments below 64; Go's non-cgo arm64 output never
   touches TLS, so writing 64 is harmless.
3. Copies the result to `/sdcard/Download/coder-native/coder` for the
   installer in `../install/setup.sh` to pick up.

## Applying

Clone coder at the pinned tag:

```sh
git clone --branch v2.34.0 --depth 1 https://github.com/coder/coder /root/build/coder
cd /root/build/coder
```

Drop the files in:

```sh
cp /path/to/coder-termux/patches/linker_exec_args.go cmd/coder/
mkdir -p androidcompat
cp /path/to/coder-termux/patches/androidcompat/*.go androidcompat/
```

Build:

```sh
bash /path/to/coder-termux/patches/build.sh
```

## Why not just use proot?

`coder ssh`, `coder ping`, and `coder port-forward` all hang inside
`proot-distro` on Pixel 8 / Android 17. Go's `net.Interfaces()` issues a
netlink request; replies are queued but `poll`/`select`/`epoll` never report
them as readable. There is no `coder` CLI flag that avoids the netlink
dependency. Running natively in Termux side-steps the proot epoll bug entirely.
