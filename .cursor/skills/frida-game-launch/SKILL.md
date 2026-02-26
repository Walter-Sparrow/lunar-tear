---
name: frida-game-launch
description: Launch NieR Re[in]carnation on Android emulator with Frida instrumentation. Use when user asks to restart the game, launch with Frida, or run with hooks. Handles adb root, frida-server, and stdin keep-alive.
---

# Frida Game Launch

## Prerequisites (after emulator cold boot)

After a cold boot, adb runs as `shell` user. Fix before launching:

```bash
export PATH="$HOME/Library/Android/sdk/platform-tools:$PATH"
adb root
sleep 2
adb shell "/data/local/tmp/frida-server &"
```

Verify: `adb shell id` should show `uid=0(root)`. `frida-ps -U` should list processes.

## Launch Command

**IMPORTANT**: Frida CLI exits immediately without a TTY. Pipe `tail -f /dev/null` to keep stdin open.

```bash
export PATH="$HOME/Library/Android/sdk/platform-tools:$PATH" && adb shell am force-stop com.square_enix.android_googleplay.nierspww && sleep 1 && tail -f /dev/null | frida -Uf com.square_enix.android_googleplay.nierspww -l /Users/kretts/FunStuff/lunar-tear/frida/hooks.js 2>&1
```

Run as background command (`block_until_ms: 0`) with `required_permissions: ["all"]`.

## After Launch

1. Do NOT sleep or poll logs automatically
2. Tell the user: "Launched. Waiting for your signal."
3. Wait for the user to say "смотри" / "look" / "check" before reading terminal output
4. When reading logs, use `Grep` for specific patterns first, then `Read` for context around matches

## Restarting

To restart, kill old frida first:

```bash
kill <previous_frida_pid> 2>/dev/null
```

Then run the launch command again. The PID is in the terminal file header.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| "need Gadget to attach on jailed Android" | frida-server not running or not root | Run `adb root && sleep 2 && adb shell "/data/local/tmp/frida-server &"` |
| Frida exits instantly with "Thank you" | No stdin/TTY | Use `tail -f /dev/null \| frida ...` pipe |
| 179ms exit, no output | adb not connected or not root | Check `adb devices` and `adb shell id` |
| Hooks load but game crashes (SIGSEGV) | Bad hook in hooks.js | Check `node --check hooks.js`, review recent changes |
