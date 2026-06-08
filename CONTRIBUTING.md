# Contributing to SkyCrypt

Before contributing to SkyCrypt, make sure you install the development environment first. If you have trouble building SkyCrypt or have any development questions, please don't hesitate to contact us on [Discord](https://discord.gg/cNgADv2kEQ)!

## Table of Contents

- [Requirements](#requirements)
- [Frontend](#frontend)
- [Installation](#installation)
  - [System Dependencies](#system-dependencies)
  - [Go Installation](#go-installation)
  - [Redis Installation](#redis-installation)
  - [MongoDB Installation](#mongodb-installation)
- [Configuration](#configuration)
- [Development](#development)
- [VS Code](#vs-code)
- [Common Issues](#common-issues)
- [Pull Requests](#pull-requests)
- [Issues](#issues)
- [License](#license)

## Requirements

- [Go](https://go.dev/) (at least 1.26)
- [Redis](https://redis.io/) (at least 7.0)
- [MongoDB](https://www.mongodb.com/) (at least 6.0)
- [Git](https://git-scm.com/) (for submodule initialization)

## Frontend

SkyCrypt-Backend, as the name suggests, is the backend of SkyCrypt. The frontend is a separate repository called [SkyCrypt-Frontend](https://github.com/SkyCryptWebsite/SkyCrypt-Frontend).

> [!IMPORTANT]
> You will need to set up the frontend in order to check out the effects of your modifications.

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

## VS Code

If you're not sure what code editor to use VS Code ([Visual Studio Code](https://code.visualstudio.com/)) is a great option. We highly recommend using it as we provide a `.vscode` folder with recommended extensions and settings which will help you with development. VS Code-like editors, like Cursor, should also work.

### Recommended Extensions

VS Code will automatically suggest the extentions we set in the `.vscode/extensions.json` file. Just go to the Extensions tab, click on the `Filter Extensions...` button, and select the `Recommended` filter. Install all the extensions that are listed there.

### Recommended Settings

VS Code will automatically use our recommended settings in the `.vscode/settings.json` file. Overriding your global settings. If you want to change any of the settings, which we don't recommend, you can do so by changing the settings in the `.vscode/settings.json` file or deleting the file altogether to use your global settings.

Please ensure that you don't accidentally commit the changes you made to the `.vscode/settings.json` file if that's not intended for your contribution. You can do this by adding the file to your `.gitignore` file.

## Common Issues

### Submodule Not Initialized

If the `NotEnoughUpdates-REPO` directory is empty:

```bash
git submodule update --init --recursive
```

## Pull Requests

When you are ready to submit your changes, please create a pull request (PR) on GitHub. Make sure to check the following:

- **Use Conventional Commits**: Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification for your commit messages.
- Your code builds successfully. Run `go build` to check.
- Your PR has a clear title and description explaining the changes you made.

### Commit Message Format

Your commit messages should follow this format:

```
<type>[optional scope]: <subject>
```

The scope is optional but recommended for better categorization.

**Examples**:

- `feat(stats): add dungeon statistics display`
- `feat: add new feature`
- `fix(ui): correct navbar alignment on mobile`
- `fix: resolve rendering issue`
- `docs: update contributing guidelines`

**Commit Type Reference**: See [conventionalcommits.org](https://www.conventionalcommits.org/en/v1.0.0/#summary) for a quick reference guide.

## Issues

If you find a bug or have a feature request, please open an issue on GitHub or on our [Discord server](https://discord.gg/cNgADv2kEQ) (preferred). When opening an issue, please provide as much information as possible, follow the issue template, and include any relevant screenshots or error messages. This will help us understand the problem and address it more quickly.

## License

By contributing to SkyCrypt, you agree that your contributions will be licensed under the [MIT License](https://github.com/SkyCryptWebsite/SkyCrypt-Backend/blob/prod/LICENSE). This means that your contributions will be open source and available for anyone to use, modify, and distribute.
