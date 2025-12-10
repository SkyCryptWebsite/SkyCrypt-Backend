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

- Go 1.25.1 or later
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
HYPIXEL_API_KEY=""
DISCORD_WEBHOOK=""
DEV="true"
ENABLE_ARMOR_HEX="false"
MONGO_URI="mongodb://localhost:27017"
MONGO_DB_NAME="SkyCrypt"
```

### Environment Variable Reference

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `HYPIXEL_API_KEY` | Your Hypixel API key. Obtain from [Hypixel Developer Portal](https://developer.hypixel.net/) | - | Yes |
| `DISCORD_WEBHOOK` | Discord webhook URL for error notifications and startup messages | - | No |
| `DEV` | Enable development mode. Set to `true` for local development | `false` | No |
| `ENABLE_ARMOR_HEX` | Enable hexadecimal armor color support | `false` | No |
| `MONGO_URI` | MongoDB connection URI | `mongodb://localhost:27017` | No |
| `MONGO_DB_NAME` | MongoDB database name | `SkyCrypt` | No |
| `REDIS_HOST` | Redis server hostname | `localhost` | No |
| `REDIS_PORT` | Redis server port | `6379` | No |
| `REDIS_PASSWORD` | Redis authentication password | - | No |

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

## Common Issues

### Submodule Not Initialized

If the `NotEnoughUpdates-REPO` directory is empty:

```bash
git submodule update --init --recursive
```

