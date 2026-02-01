# Injective CLI

NPM package wrapper for the Injective blockchain node binary (`injectived`).

## Installation

```bash
npm install -g injective-cli
```

Or use with npx (no installation required):

```bash
npx -p injective-cli injectived --help
```

## Supported Platforms

- macOS ARM64 (Apple Silicon)
- Linux ARM64
- Linux x64

## Usage

After installation, the `injectived` command is available:

```bash
injectived --help
injectived init
injectived start
```

## How It Works

This package uses platform-specific optional dependencies to download the correct binary for your system. If optional dependencies are disabled, it will fall back to downloading the binary during installation.

## Platform-Specific Packages

If you need to depend on a specific platform version:

- `injective-cli-darwin-arm64`
- `injective-cli-linux-arm64`
- `injective-cli-linux-x64`

## License

BUSL-1.1
