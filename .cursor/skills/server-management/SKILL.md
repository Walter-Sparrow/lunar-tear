---
name: server-management
description: Start, stop, and restart the Lunar Tear game server safely. Use when user asks to restart the server, start the server, or stop the server. NEVER kill processes on system ports (80, 443).
---

# Server Management

## CRITICAL SAFETY RULES

1. **NEVER run `lsof -ti:80` or `lsof -ti:443` with `kill`** — ports 80 and 443 are used by system services, proxies, and browsers. Killing them breaks internet and crashes apps.
2. **Always kill by PID**, never by port scan on system ports.
3. The server binary `./lunar-tear` listens on ports 80, 443, 7777, and 8080. To stop it, kill the specific process.

## Finding the Server PID

Option 1 — from terminal file header:
```bash
head -6 /path/to/terminal/file.txt   # pid is on line 2
```

Option 2 — by process name:
```bash
pgrep -f lunar-tear
```

Option 3 — by non-system port (7777 or 8080 are safe to query):
```bash
lsof -ti:7777
```

## Starting the Server

```bash
cd /Users/kretts/FunStuff/lunar-tear/server && ./lunar-tear 2>&1
```

Run as background command (`block_until_ms: 0`) with `required_permissions: ["all"]`.

Verify startup by checking for these log lines:
```
Octo HTTP server listening on :8080
HTTPS server listening on :443
HTTP server also listening on :80
gRPC server listening on :7777
```

## Stopping the Server

```bash
# Safe: find by process name
pgrep -f '\./lunar-tear$' | xargs kill 2>/dev/null
```

Or if you know the PID:
```bash
kill <pid> 2>/dev/null
```

**WARNING**: Do NOT use `lsof -ti:7777 | xargs kill` — the emulator also listens on 7777 and will be killed!

**NEVER DO THIS:**
```bash
# DANGEROUS — kills system services!
lsof -ti:80 -ti:443 | xargs kill -9
```

## Restarting the Server

```bash
pgrep -f '\./lunar-tear$' | xargs kill 2>/dev/null; sleep 2 && cd /Users/kretts/FunStuff/lunar-tear/server && ./lunar-tear 2>&1
```

Run as background command (`block_until_ms: 0`) with `required_permissions: ["all"]`.

## Rebuilding the Server

If Go source files changed, rebuild before restarting:

```bash
pgrep -f '\./lunar-tear$' | xargs kill 2>/dev/null; sleep 2 && cd /Users/kretts/FunStuff/lunar-tear/server && go build -o lunar-tear ./cmd/lunar-tear/ && sleep 1 && ./lunar-tear 2>&1
```

## Full Restart (Server + Game)

When both need restarting (e.g., after server code changes):

1. Kill old frida process (PID from terminal header)
2. Kill server: `pgrep -f '\./lunar-tear$' | xargs kill 2>/dev/null`
3. Wait: `sleep 2`
4. Start server (background)
5. Wait for server "listening" logs
6. Launch game with Frida (see frida-game-launch skill)

## Ports Reference

| Port | Service | Safe to `lsof -ti` + kill? |
|------|---------|---------------------------|
| 7777 | gRPC server | **NO** — emulator also uses this port! Use `pgrep -f lunar-tear` instead |
| 8080 | Octo HTTP | YES — our process only |
| 80   | HTTP (game web) | **NO** — system services use this |
| 443  | HTTPS | **NO** — system services use this |
