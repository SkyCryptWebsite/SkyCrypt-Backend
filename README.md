<p align="center">
  <picture>
    <source media="(prefers-color-scheme: light)" srcset="static/img/logo_black.avif">
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
  - [MongoDB Installation](#mongodb-installation)
- [Configuration](#configuration)
- [Development](#development)
- [Common Issues](#common-issues)

## Requirements

- Go 1.26 or later
- Redis 7.0 or later
- MongoDB 6.0 or later
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

### MongoDB Installation

Install MongoDB from the AUR. Using an AUR helper such as `yay`:

```bash
yay -S mongodb-bin
```

Alternatively, build from source:

```bash
git clone https://aur.archlinux.org/mongodb-bin.git
cd mongodb-bin
makepkg -si
```

Enable and start the MongoDB service:

```bash
sudo systemctl enable mongodb
sudo systemctl start mongodb
```

Verify MongoDB is running:

```bash
mongosh --eval "db.runCommand({ ping: 1 })"
```

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

# MongoDB connection.
MONGO_URI="mongodb://localhost:27017"
MONGO_DB_NAME="SkyCrypt"

# Redis connection.
REDIS_HOST="localhost"
REDIS_PORT="6379"
REDIS_PASSWORD=""

# Server-to-server API protection for private API routes.
SERVER_API_TOKEN="DuckySoLuckyWasHere"
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
| `MONGO_URI` | MongoDB connection URI | `mongodb://localhost:27017` | No |
| `MONGO_DB_NAME` | MongoDB database name | `SkyCrypt` | No |
| `REDIS_HOST` | Redis server hostname | `localhost` | No |
| `REDIS_PORT` | Redis server port | `6379` | No |
| `REDIS_PASSWORD` | Redis authentication password | - | No |
| `SERVER_API_TOKEN` | Shared token required by protected API routes through the `X-API-Token` header | - | Yes in production |
| `DISABLE_SERVER_API_AUTH` | Disable `X-API-Token` checks for local development only | `false` | No |

Protected API routes return `401 Unauthorized` unless the request includes `X-API-Token: <SERVER_API_TOKEN>`. Keep `DISABLE_SERVER_API_AUTH` unset or `false` outside local development.

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

Enable the repository git hooks so `swag init` runs automatically before each push:

```bash
git config core.hooksPath .githooks
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

The public API exposes source and license metadata at `/api/source`.

## Common Issues

### Submodule Not Initialized

If the `NotEnoughUpdates-REPO` directory is empty:

```bash
git submodule update --init --recursive
```
