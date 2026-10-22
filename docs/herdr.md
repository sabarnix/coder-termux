# Running herdr on a Coder workspace

[Herdr](https://herdr.dev) is a tmux-like persistent-session terminal workspace
manager. The install script explicitly rejects Android/Termux, and the
published `linux-aarch64` build is a statically linked non-PIE ELF — Bionic's
linker-exec won't load it, and the e_type patch trick that works for the
`coder` binary doesn't help here because the code isn't position-independent.

The practical workflow:

- Install the x86_64 build on your Coder workspace (`herdr.dev/install.sh`).
- From Termux, launch a plain SSH session that attaches to it:

  ```sh
  ssh -t <workspace>.coder /home/<user>/.local/bin/herdr
  ```

Herdr owns the session on the server. A dropped SSH just means re-running
the command to reattach; no scrollback or state is lost.

A convenient alias to drop in `~/.bashrc`:

```sh
alias wrk='ssh -t <workspace>.coder /home/<user>/.local/bin/herdr'
```

Or source `client/profile.d/coder.sh` from this repo after setting
`CODER_WORKSPACE`.

## Why not mosh for persistence?

Mosh plus the included `mosh-coder` wrapper does work, but:

- Mosh clears the scrollback buffer.
- Coder's `port-forward --udp` has "TCP-like semantics" and dropped roughly
  4/5 synthetic packets on a cold connection in testing.
- Session persistence lives in `mosh-server`'s memory — if that dies, you
  lose state.

Herdr handles persistence on the server, has its own scrollback, and runs
over plain TCP SSH, which Coder's native tunnel handles cleanly.
