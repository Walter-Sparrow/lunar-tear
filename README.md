# Lunar Tear

Private server research project for **NieR Re[in]carnation**.

## Description

Lunar Tear emulates key game backend services so a patched client can connect to your local/private server instead of official endpoints.

Current project direction:

- Keep client changes minimal (prefer static APK patching over heavy runtime hooks).
- Implement missing game behavior on the server.
- Use Frida mainly for investigation/debugging, not as a permanent runtime dependency.

Main components:

- `server/` - Go gRPC + HTTP services.
- `scripts/` - APK patching and data tooling.
- `frida/` - instrumentation scripts for reverse engineering and runtime tracing.
- `docs/` - active progress and investigation notes.

## How To Patch An APK

This repository includes a static patcher: `scripts/patch_apk.py`.

It patches:

1. `global-metadata.dat` URL/hostname strings
2. `libil2cpp.so` (SSL bypass + crypto passthrough + Octo list behavior)
3. `AndroidManifest.xml` (`networkSecurityConfig`)
4. `res/xml/network_security_config.xml` (allow cleartext HTTP)

### Prerequisites

- Python 3
- `apktool`
- Android build tools (`apksigner`, `zipalign`)
- A decompiled APK directory (from `apktool d`)

### Steps

1. Decompile APK:

```bash
apktool d client/3.7.1.apk -o client/patched
```

2. Run patcher (example for emulator host):

```bash
python3 scripts/patch_apk.py client/patched --server-ip 10.0.2.2 --http-port 8080 --grpc-port 7777
```

3. Rebuild:

```bash
apktool b client/patched -o client/patched.apk
```

4. Generate signing key (one-time):

```bash
keytool -genkeypair \
  -v \
  -keystore client/debug.keystore \
  -alias androiddebugkey \
  -storepass android \
  -keypass android \
  -keyalg RSA \
  -keysize 2048 \
  -validity 10000 \
  -dname "CN=Android Debug,O=Android,C=US"
```

5. Align + sign:

```bash
zipalign -f 4 client/patched.apk client/patched-aligned.apk
apksigner sign --ks client/debug.keystore --ks-pass pass:android client/patched-aligned.apk
```

Notes:

- `--server-ip` replacement must fit existing metadata string lengths (the script validates this).
- The patcher rewrites gRPC host to host-only; server can also listen on `:443` for this flow.

## How To Setup Frida

Frida is mainly for local debugging/probing.

### Prerequisites

- Android emulator/device with `adb`
- `frida-tools` on host
- `frida-server` binary on device (commonly `/data/local/tmp/frida-server`)

### Setup Steps

1. Start `frida-server` on device:

```bash
adb shell "su -c /data/local/tmp/frida-server" &
```

2. Launch game with a script:

```bash
tail -f /dev/null | frida -Uf com.square_enix.android_googleplay.nierspww -l frida/hooks.js
```

Useful references:

- `docs/FRIDA_GUIDE.md`
- `frida/hooks.js`

## How To Launch A Server

The server entrypoint is `server/cmd/lunar-tear/main.go`.

### Prerequisites

- Go `1.24+`

### Run

```bash
cd server
go run ./cmd/lunar-tear \
  --grpc-port 7777 \
  --http-port 8080 \
  --host 10.0.2.2 \
  --bootstrap-profile fresh
```

Default behavior:

- gRPC listens on `:7777` (and also tries `:443`).
- HTTP asset/API server listens on `:8080` (and also tries `:80`).

Important flags:

- `--host` - hostname/IP given to the client.
- `--grpc-port` - main gRPC port.
- `--http-port` - HTTP/Octo port.
- `--bootstrap-profile` - initial user state profile (`fresh`, `main-quest-scene-9`).

### Optional: Regenerate protobuf stubs

```bash
cd server
make proto
```

## Development Notes

- Current trusted project status: `docs/PROGRESS.active.md`
- Deprecated history: `docs/PROGRESS.md`

## Current Working Workflow

Current proven in-game flow:

1. Game starts
2. User enters name
3. 2D story scene
4. 3D story scene
5. Trailer
6. `"Showdown in the Wastes"` quest with battle
