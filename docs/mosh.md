# Mosh over `coder port-forward --udp`

`client/mosh-coder` is a thin bash wrapper that lets you run Mosh end to end
through Coder's native tunnel without relying on direct UDP reachability.

## How it works

1. `ssh <workspace>.coder "mosh-server new -s -p <port>"` — starts a server
   on a random port in the 60200–60299 range. The server prints
   `MOSH CONNECT <port> <base64 key>`; the wrapper parses the line.
2. `coder port-forward <workspace> --udp <port>:<port>` runs in the background.
   The wrapper polls its log for the "Ready!" line (usually around 6 seconds)
   before continuing.
3. `MOSH_KEY=<key> mosh-client 127.0.0.1 <port>` is `exec`'d. On exit, the
   trap kills the background port-forward.

## Caveats

- Coder's UDP forward documents "TCP-like semantics to support stateful UDP
  protocols." In loopback testing, roughly 4 of 5 synthetic packets were
  dropped on a cold connection. Mosh's own retransmit absorbs this, but the
  first second or two of a session can feel sticky.
- Mosh's scrollback is zero. Pair this with `tmux`, `screen`, or herdr on the
  remote if you want to scroll back through output. See `docs/herdr.md`.
- Touch-swipe inside Termux is converted to arrow-key sequences while any
  app is in the alternate screen (including mosh), because Termux has no
  concept of mouse events for touch. This is a Termux behavior, not a Mosh one.

## Usage

```sh
mosh-coder <workspace-name>
```
