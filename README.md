# port-who

[![ci](https://github.com/frozenmango8/port-who/actions/workflows/ci.yml/badge.svg)](https://github.com/frozenmango8/port-who/actions/workflows/ci.yml)

Show which process is listening on a port.

## Install

```
go install github.com/frozenmango8/port-who@latest
```

`goreleaser release` builds linux, macOS, and Windows binaries. Config is in `.goreleaser.yaml`.

## Usage

With no port, every listening TCP socket and every bound UDP socket is listed.

```
$ port-who 3000
PID   USER    PROTO  ADDRESS       NAME     COMMAND
8726  ubuntu  tcp    0.0.0.0:3000  python3  python3 -m http.server 3000
```

```
$ port-who --json 3000
[{"pid":8726,"name":"python3","cmdline":"python3 -m http.server 3000","user":"ubuntu","proto":"tcp","addr":"0.0.0.0:3000"}]
```

`--kill` sends SIGTERM on Unix and `taskkill` on Windows. It asks first. `-y` skips the prompt. If stdin is not a terminal, nothing is signalled unless `-y` is passed.

```
$ port-who --kill 3000
terminate 8726 (python3)? [y/N] y
sent SIGTERM to 8726
```

Exit status is 1 when the port has no listener.

Sockets you are not allowed to inspect still appear, without a pid:

```
$ port-who 2375
port-who: some sockets belong to other users; rerun as root to see the process
PID  USER  PROTO  ADDRESS       NAME  COMMAND
-    root  tcp    0.0.0.0:2375  -
```

## Platforms

Linux reads `/proc/net/tcp`, `/proc/net/tcp6`, `/proc/net/udp`, and `/proc/net/udp6`, then matches the socket inode against `/proc/<pid>/fd`. The uid in the table is used when the fd directory is not readable.

macOS shells out to `lsof -nP -F` and `ps`. `lsof` omits other users' processes unless it is allowed to see them. A missing `lsof` is an error.

Windows shells out to `netstat -ano` and `tasklist`. Command lines come from `Win32_Process` via PowerShell. If PowerShell fails, the name from tasklist is still printed.

## Limitations

- Established TCP connections are ignored, as are UDP sockets that are connected to a peer.
- On Linux the name column is `/proc/<pid>/comm`, which the kernel truncates at 15 bytes. The command column is the full command line.
- `--kill` does not wait for the process to exit, and it does not follow up with SIGKILL or `taskkill /F`.
- Windows parsing uses column order and numeric addresses. It does not depend on the localized state word.
