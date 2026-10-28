<p align="center">
  <picture>
    <source media="(prefers-color-scheme: light)" srcset="https://github.com/SkyCryptWebsite/SkyCrypt-Frontend/raw/dev/static/img/logo_black.avif">
    <img alt="SkyCrypt" height="96px" src="https://github.com/SkyCryptWebsite/SkyCrypt-Frontend/raw/dev/static/img/logo.avif">
  </picture>
</p>
<h1 align="center">SkyCrypt Backend</h1>
<h3 align="center">A Hypixel SkyBlock Profile Viewer</h3>

<p align="center">
  <a href="https://www.patreon.com/shiiyu"><img alt="Sponsor" src="https://img.shields.io/badge/sponsor-30363D?style=for-the-badge&logo=GitHub-Sponsors&logoColor=#white" /></a>
  &nbsp;
  <a href="https://github.com/SkyCryptWebsite/SkyCrypt-Backend/stargazers"><img alt="GitHub Repo stars" src="https://img.shields.io/github/stars/SkyCryptWebsite/SkyCrypt-Backend?style=for-the-badge" /></a>
</p>

A high-performance Go backend API for [SkyCrypt](https://github.com/SkyCryptWebsite/SkyCrypt), providing statistics and data processing for Hypixel SkyBlock players.

Originally inspired by [LeaPhant's skyblock-stats](https://github.com/LeaPhant/skyblock-stats).

**Website**: https://sky.shiiyu.moe \
**Development Website**: https://cupcake.shiiyu.moe

**Frontend**: [SkyCrypt-Frontend](https://github.com/SkyCryptWebsite/SkyCrypt-Frontend)

## Table of Contents

- [Requirements](#requirements)
- [Installation](#installation)
  - [System Dependencies](#system-dependencies)
  - [Go Installation](#go-installation)
  - [Redis Installation](#redis-installation)
  - [PostgreSQL Installation](#postgresql-installation)
- [Configuration](#configuration)
- [Development](#development)
- [Common Issues](#common-issues)

## Requirements

- Go 1.26 or later
- Redis 7.0 or later
- PostgreSQL 14 or later (optional)
- Git (for submodule initialization)

## Installation

The following instructions are written for Arch Linux. Adjust package manager commands accordingly for other distributions.

### System Dependencies

Update your system and install essential build tools:

```bash
sudo pacman -Syu
sudo pacman -S base-devel git
```

### Go Installation

Install Go from the official Arch Linux repositories:

```bash
sudo pacman -S go
```

Verify the installation:

```bash
go version
```

### Redis Installation

Install Redis:

```bash
sudo pacman -S redis
```

Enable and start the Redis service:

```bash
sudo systemctl enable redis
sudo systemctl start redis
```

Verify Redis is running:

```bash
redis-cli ping
```

Expected output: `PONG`

### PostgreSQL Installation

PostgreSQL stores the fallback cache that is served when Hypixel or Mojang is unavailable. It is optional for local development.

```bash
sudo pacman -S postgresql
sudo -iu postgres initdb -D /var/lib/postgres/data
sudo systemctl enable --now postgresql
sudo -iu postgres createuser --pwprompt skycrypt
sudo -iu postgres createdb -O skycrypt skycrypt_backend
```

With Docker, reuse the PostgreSQL container from the frontend's `docker-compose.yml` and create a `skycrypt_backend` database in it. The backend's `docker compose up -d` starts Redis only.

## Configuration

### Environment Variables

Create a `.env` file in the project root directory. Use `.env.example` as a template:

```bash
cp .env.example .env
```

Edit the `.env` file with your configuration:

```dotenv
# Hypixel API key for fetching player, profile, museum, and Garden data.
HYPIXEL_API_KEY=""

# Optional Discord webhook for startup and error notifications.
DISCORD_WEBHOOK=""

# Local development mode. Processed route responses bypass RAM and Redis caching,
# while raw upstream API data caches remain enabled. Production deployments should leave this false.
DEV="true"
SKYCRYPT_PREFORK="false"

# Rendering and diagnostics.
ENABLE_ARMOR_HEX="false"
VERBOSE_LOGGING="true"
FORENSICS_ENABLED="true"
LOG_STDOUT="0"

# Public backend origin used when building rendered asset URLs.
DOMAIN="http://localhost:8080"

# Commit hash exposed by /api/source. Docker builds set this automatically.
SOURCE_COMMIT=""

# PostgreSQL connection for the fallback cache. Leave empty to disable it.
DATABASE_URL="postgres://skycrypt:DEV@localhost:5432/skycrypt_backend"

# Redis connection.
REDIS_HOST="localhost"
REDIS_PORT="6379"
REDIS_PASSWORD=""

# Shared token between frontend and backend, and the frontend origin used to verify API keys.
SERVER_API_TOKEN="DuckySoLuckyWasHere"
FRONTEND_INTERNAL_URL="http://localhost:5173"
DISABLE_SERVER_API_AUTH="true"
```

### Environment Variable Reference

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `HYPIXEL_API_KEY` | Your Hypixel API key. Obtain from [Hypixel Developer Portal](https://developer.hypixel.net/) | - | Yes |
| `DISCORD_WEBHOOK` | Discord webhook URL for error notifications and startup messages | - | No |
| `DEV` | Enable development mode. When `true`, processed route responses bypass RAM and Redis caching while raw upstream API data caches remain enabled | `false` | No |
| `SKYCRYPT_PREFORK` | Explicitly enable or disable Fiber prefork. Defaults to `false` when `DEV=true`, otherwise `true` | `false` in development, `true` otherwise | No |
| `ENABLE_ARMOR_HEX` | Enable hexadecimal armor color support | `false` | No |
| `VERBOSE_LOGGING` | Enable extra debug logging from utility helpers | `false` | No |
| `FORENSICS_ENABLED` | Enable forensic request/performance logging and `/api/forensics` endpoints | `false` | No |
| `LOG_STDOUT` | Also write forensic JSON logs to stdout when set to `1` | `0` | No |
| `DOMAIN` | Public backend origin used for generated image/resource URLs | `https://sky.shiiyu.moe` | No |
| `SOURCE_COMMIT` | Git commit hash exposed by `/api/source`; normally injected by Docker builds | - | No |
| `DATABASE_URL` | PostgreSQL connection URL for the fallback cache | - | No |
| `REDIS_HOST` | Redis server hostname | `localhost` | No |
| `REDIS_PORT` | Redis server port | `6379` | No |
| `REDIS_PASSWORD` | Redis authentication password | - | No |
| `REDIS_MAXMEMORY` | Redis maximum memory when using the bundled Compose deployment | `8gb` | No |
| `REDIS_MAXMEMORY_POLICY` | Redis eviction policy when the maximum is reached | `allkeys-lfu` | No |
| `SERVER_API_TOKEN` | Shared token the frontend sends in `X-API-Token` for full access; also authenticates the backend to the frontend's internal routes | - | Yes in production |
| `FRONTEND_INTERNAL_URL` | Frontend origin reachable from the backend, used to verify API keys and report their usage | - | Yes in production |
| `DISABLE_SERVER_API_AUTH` | Disable API key and `X-API-Token` checks for local development only | `false` | No |

Profile routes need either the frontend's `X-API-Token: <SERVER_API_TOKEN>` or a user's SkyCrypt API key in `x-api-key`. Keys are Better Auth API keys managed by the frontend:

- The backend verifies a key through the frontend's `POST /api/internal/api-keys/verify` and caches the result for a minute, so a disabled key stops working within a minute.
- Each key's `rateLimitMax` per `rateLimitTimeWindow` is enforced in Redis. Every 15 seconds the backend writes each key's request count for its current window back to the frontend (`POST /api/internal/api-keys/usage`), so `requestCount` in the admin dashboard shows real usage, rejected requests included.
- Keys only get cached data (Redis, then the Postgres fallback cache) and get `404 NOT_CACHED` otherwise, because Hypixel's policy doesn't allow proxying its API. Keys with the `cache-bypass` scope, and the frontend, may fetch from Hypixel.

Keep `DISABLE_SERVER_API_AUTH` unset or `false` outside local development.

## Development

Clone the repository with submodules:

```bash
git clone --recurse-submodules https://github.com/SkyCryptWebsite/SkyCrypt-Backend.git
cd SkyCrypt-Backend
```

If you have already cloned the repository without submodules:

```bash
git submodule update --init --recursive
```

Download Go dependencies:

```bash
go mod download
```

Run the application:

```bash
go run main.go
```

## Licensing

SkyCrypt Backend uses a split license model:

| Material | License | Notes |
| --- | --- | --- |
| SkyCrypt-owned source code, documentation, configuration, generated API docs, and build scripts introduced or modified from the June 2026 license-change commit onward | [GNU AGPLv3](./LICENSE) | Network use of modified versions must provide users access to the corresponding source code. |
| Third-party Go dependencies | Their respective licenses | See upstream module metadata and `go.sum`. |
| FurfSky Reborn resource-pack assets | Upstream FurfSky Reborn terms | These assets are not relicensed under GNU AGPLv3. See [`NOTICE`](./NOTICE) and [`REUSE.toml`](./REUSE.toml). |
| Hypixel Plus resource-pack assets | CC-BY-NC-ND-4.0 | These assets are not relicensed under GNU AGPLv3. See [`NOTICE`](./NOTICE) and [`REUSE.toml`](./REUSE.toml). |
| Minecraft/Mojang-derived assets and rendering helper assets | Upstream Mojang/Microsoft terms | These assets are not relicensed under GNU AGPLv3. See [`NOTICE`](./NOTICE) and [`REUSE.toml`](./REUSE.toml). |
| Asset files pending provenance review | Their respective rights holders | These assets are not relicensed under GNU AGPLv3 by default. |
| SkyCrypt brand/logo assets (`assets/static/favicon.ico`, `assets/static/skycrypt-api.avif`) | [All rights reserved](./LICENSES/LicenseRef-SkyCrypt-Brand.txt) | Use only, unmodified, to refer or link to SkyCrypt. Deployments of SkyCrypt must use their own branding. See [`NOTICE`](./NOTICE). |

The public API exposes source and license metadata at `/api/source`.

## Common Issues

### Submodule Not Initialized

If the `NotEnoughUpdates-REPO` directory is empty:

```bash
git submodule update --init --recursive
```
