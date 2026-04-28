# Lunar Tear

Private server research project for a certain discontinued mobile game.
Discord server: https://discord.gg/MZAf5aVkJG

## How To Launch The Server

### Prerequisites

- Go 1.25+
- [goose](https://github.com/pressly/goose) migration tool
- Populated `server/assets/` directory

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### Quick Start (Wizard)

The interactive wizard walks you through setup with a few simple questions — no flags or networking knowledge needed. It auto-detects the right IP address for your emulator or phone and launches all services.

```bash
cd server
go run ./cmd/wizard
```

Your choices are saved so next time you just press Enter to relaunch with the same settings. To skip the confirmation prompt entirely (useful for scripts or quick relaunches), pass `--prefer-saved`:

```bash
go run ./cmd/wizard --prefer-saved
```

If no saved config exists, the flag prints an error and exits.

#### Custom Ports

By default the wizard uses ports 8003 (gRPC), 8080 (CDN), and 3000 (auth). Override any of them with flags:

```bash
go run ./cmd/wizard --grpc-port 9003 --cdn-port 9080
```

| Flag             | Default | Description                        |
| ---------------- | ------- | ---------------------------------- |
| `--prefer-saved` | `false` | Reuse saved config without prompting |
| `--grpc-port`    | `8003`  | gRPC server port                   |
| `--cdn-port`     | `8080`  | CDN server port                    |
| `--auth-port`    | `3000`  | Auth server port                   |

Custom ports are saved to `.wizard.json` alongside your other settings. On the next run the saved ports are reused automatically — no need to pass the flags again. If you later pass different port flags, the wizard warns you that the ports changed and asks for confirmation before continuing.

### Regenerate protobuf stubs

```bash
cd server
make proto
```

### Database

Player state is stored in a SQLite database. Run migrations before starting the server:

```bash
cd server
make migrate
```

Or manually:

```bash
cd server
mkdir -p db
goose -dir migrations -allow-missing sqlite3 db/game.db up
```

### Importing a Snapshot

To import a JSON snapshot into the database, use the import tool. The `--uuid` flag must match the UUID your game client sends during authentication:

```bash
cd server
make import SNAPSHOT=snapshots/scene_1.json UUID=<your-client-uuid>
```

Or directly:

```bash
go run ./cmd/import-snapshot \
  --snapshot snapshots/scene_1.json \
  --uuid <your-client-uuid> \
  --db db/game.db
```

| Flag         | Default      | Description                                   |
| ------------ | ------------ | --------------------------------------------- |
| `--snapshot` | *(required)* | Path to JSON snapshot file                     |
| `--uuid`     | *(required)* | UUID to assign (must match the client's UUID)  |
| `--db`       | `db/game.db` | SQLite database path                           |

### Run

The server is split into two binaries: a gRPC game server and an HTTP asset CDN. Both must be running for the client to work.

**Start the CDN** (serves asset bundles, list.bin, master data, web pages):

```bash
cd server
go run ./cmd/octo-cdn \
  --listen 0.0.0.0:8080 \
  --public-addr 10.0.2.2:8080
```

**Start the game server** (gRPC, points the client at the CDN):

```bash
cd server
go run ./cmd/lunar-tear \
  --listen 0.0.0.0:8003 \
  --public-addr 10.0.2.2:8003 \
  --octo-url http://10.0.2.2:8080
```

The default listen address is `0.0.0.0:443`, which requires `sudo` (privileged port). Use `--listen` with a high port to avoid this. If you do need port 443, either use `sudo` or grant the binary the capability on Linux:

```bash
go build -o lunar-tear ./cmd/lunar-tear
sudo setcap cap_net_bind_service=+ep ./lunar-tear
./lunar-tear --public-addr 10.0.2.2:443 --octo-url http://10.0.2.2:8080
```

The CDN can run on a completely separate machine — just set `--octo-url` on the game server and `--public-addr` on the CDN to the externally-reachable address.

### Run All Services At Once

Instead of starting each service individually, use the dev runner to launch all three (auth, CDN, game server) with a single command. No Docker required — works on macOS, Linux, and Windows.

```bash
cd server
make dev
```

Or directly:

```bash
cd server
go run ./cmd/dev
```

Each service's output is prefixed with a colored label (`[auth]`, `[cdn]`, `[grpc]`). Press Ctrl+C to shut everything down.

The dev runner automatically builds each service into `bin/` before launching. This means the binaries have stable file paths, so **Windows Firewall only prompts once** — subsequent runs reuse the same allowed executables. The wizard performs the same build step transparently.

Override defaults with namespaced flags:

```bash
go run ./cmd/dev --grpc.listen 0.0.0.0:9000 --grpc.public-addr 10.0.2.2:9000 --cdn.public-addr 192.168.1.50:8080
```

Or via `make`:

```bash
make dev ARGS="--grpc.listen 0.0.0.0:9000 --grpc.public-addr 10.0.2.2:9000"
```

| Flag                  | Default            | Description                              |
| --------------------- | ------------------ | ---------------------------------------- |
| `--auth.listen`       | `0.0.0.0:3000`     | auth-server listen address               |
| `--auth.db`           | `db/auth.db`       | auth-server SQLite database path         |
| `--cdn.listen`        | `0.0.0.0:8080`     | octo-cdn local bind address              |
| `--cdn.public-addr`   | `10.0.2.2:8080`    | octo-cdn externally-reachable addr       |
| `--grpc.listen`       | `0.0.0.0:8003`     | lunar-tear gRPC listen address           |
| `--grpc.public-addr`  | `10.0.2.2:8003`    | lunar-tear externally-reachable addr     |
| `--grpc.octo-url`     | `http://10.0.2.2:8080` | Octo CDN base URL passed to lunar-tear |
| `--grpc.auth-url`     | `http://localhost:3000` | auth server base URL passed to lunar-tear |
| `--no-color`          | `false`            | disable colored output                   |

### Ports

| Protocol | Port | Binary        | Notes                                                       |
| -------- | ---- | ------------- | ----------------------------------------------------------- |
| gRPC     | 443  | `lunar-tear`  | default; configurable with `--listen` (requires patched client) |
| HTTP     | 8080 | `octo-cdn`    | Octo asset API + game web pages                             |

### Game Server Flags (`lunar-tear`)

| Flag            | Default           | Description                                          |
| --------------- | ----------------- | ---------------------------------------------------- |
| `--listen`      | `0.0.0.0:443`     | gRPC listen address (host:port)                      |
| `--public-addr` | `127.0.0.1:443`   | externally-reachable host:port advertised to clients  |
| `--octo-url`    | *(required)*      | CDN base URL the client uses for assets (e.g. `http://10.0.2.2:8080`) |
| `--db`          | `db/game.db`      | SQLite database path                                 |
| `--auth-url`    | *(empty)*         | Auth server base URL (e.g. `http://localhost:3000`)  |

### CDN Flags (`octo-cdn`)

| Flag            | Default           | Description                                              |
| --------------- | ----------------- | -------------------------------------------------------- |
| `--listen`      | `0.0.0.0:8080`    | local bind address                                       |
| `--public-addr` | `127.0.0.1:8080`  | externally-reachable address (used in list.bin rewriting) |
| `--assets-dir`  | `.`               | root directory containing the `assets/` tree             |

### Docker

Three services are available via Docker Compose: the game server (`lunar-tear`), the CDN (`octo-cdn`), and the auth server (`auth-server`). Migrations run automatically on game server start.

```bash
cd server
docker compose up -d
```

The `db/` directory is mounted as a volume so both `game.db` and `auth.db` persist across restarts. Make sure `assets/` is populated before starting.

Each service has its own image and can be deployed independently:

| Service  | Image                       | Default Port | Notes                          |
| -------- | --------------------------- | ------------ | ------------------------------ |
| `server` | `kretts/lunar-tear:latest`  | 8003         | gRPC game server               |
| `cdn`    | `kretts/octo-cdn:latest`    | 8080         | HTTP asset CDN                 |
| `auth`   | `kretts/auth-server:latest` | 3000         | Account registration and login |

The game server is configured via environment variables in the compose file: `LUNAR_LISTEN` (bind address), `LUNAR_PUBLIC_ADDR` (client-facing address), `LUNAR_OCTO_URL`, and `LUNAR_AUTH_URL`. Auth is optional — if `LUNAR_AUTH_URL` is unset the game server starts without it.

### Makefile Targets

All targets run from the `server/` directory.

| Target         | Description                                             |
| -------------- | ------------------------------------------------------- |
| `make proto`   | Regenerate protobuf stubs                               |
| `make build`   | Build the game server binary                            |
| `make build-cdn` | Build the CDN binary                                  |
| `make build-auth` | Build the auth server binary                          |
| `make build-dev` | Build the dev runner binary to `bin/`                  |
| `make build-all` | Build all service binaries to `bin/`                   |
| `make build-import` | Build the import-snapshot tool                     |
| `make build-claim-account` | Build the claim-account tool                |
| `make clean`   | Remove the `bin/` directory                              |
| `make dev`     | Run all three services with one command                  |
| `make migrate` | Run goose migrations on `db/game.db`                    |
| `make import`  | Import a snapshot (`SNAPSHOT=... UUID=...` required)     |

## Claim Account

Transfers an existing game account to the most recently connected client. Looks up a player by their in-game name, assigns the new client's UUID to that account, and deletes the empty account the new client created.

Useful when a new client connects and creates a throwaway account, but you want it to load an existing account instead.

```bash
cd server
go run ./cmd/claim-account --name "PlayerName" --db db/game.db
```

| Flag     | Default      | Description                                          |
| -------- | ------------ | ---------------------------------------------------- |
| `--name` | *(required)* | In-game player name to claim                         |
| `--db`   | `db/game.db` | SQLite database path                                 |

## Auth Server

A separate HTTP server that handles player account registration and login. The patched client's Facebook login button is redirected to this server, which presents a username/password form. Tokens issued here are validated by the game server to link or recover accounts.

### Run

```bash
cd server
go run ./cmd/auth-server \
  --listen 0.0.0.0:3000 \
  --db db/auth.db
```

The `--secret` flag accepts a hex-encoded HMAC key. If omitted, a random key is generated on startup and printed to the console — pass it back on the next restart to keep existing tokens valid.

### Flags

| Flag       | Default         | Description                                  |
| ---------- | --------------- | -------------------------------------------- |
| `--listen` | `0.0.0.0:3000`  | HTTP listen address (host:port)              |
| `--db`     | `db/auth.db`    | SQLite database path for auth users          |
| `--secret` | *(generated)*   | Hex-encoded HMAC secret for token signing    |

## ⚠️ Legal Disclaimer

**Lunar Tear** is a fan-made, non-commercial **preservation and research project** dedicated to keeping a certain discontinued mobile game playable for educational and archival purposes.

- This project is **not affiliated with**, **endorsed by**, or **approved by** the original publisher or any of its subsidiaries.
- All trademarks, copyrights, and intellectual property related to the original game and its associated franchises belong to their respective owners.
- All code in this repository is original work developed through clean-room reverse engineering for interoperability with the game client.
- No copyrighted game assets, binaries, or master data are distributed in this repository.

**Use at your own risk.** The author assumes no liability for any damages or legal consequences that may arise from using this software. By using or contributing to this project, you are solely responsible for ensuring your usage complies with all applicable laws in your jurisdiction.

This project is released under the [MIT License](LICENSE).

**If you are a rights holder with concerns regarding this project**, please contact me directly.
