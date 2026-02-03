# Distribution Packages for Injective Core

This directory contains the build infrastructure for distributing the `injectived` binary through NPM and PyPI package registries using Docker Buildx.

## Overview

The distribution system uses Docker Buildx to build packages in isolated containers. This ensures:

- **Reproducible builds**: Same environment every time
- **No host dependencies**: Only Docker is required
- **Cross-platform support**: Build for multiple architectures

Supported platforms:

- **macOS ARM64** (Apple Silicon - M1/M2/M3)
- **macOS x64** (Intel)
- **Linux ARM64**
- **Linux x64** (AMD64)

## Directory Structure

```
packaging/
├── AGENTS.md                 # This file
├── Dockerfile                # Docker build definition
├── docker-bake.hcl           # Buildx bake configuration
├── Makefile                  # Build and publish commands
├── binaries/                 # Pre-built binaries (input to builds)
├── npm/
│   ├── package.json.tmpl     # Template for platform-specific packages
│   └── injective-cli/        # Base NPM package (injective-cli wrapper)
│       ├── package.json
│       ├── tsconfig.json
│       ├── src/
│       │   ├── index.ts      # Main entry point
│       │   └── install.ts    # Post-install fallback script
│       ├── lib/              # Compiled JavaScript (generated)
│       ├── README.md
│       └── LICENSE
└── pypi/
    └── injective-cli/        # PyPI package (injective-cli)
        ├── pyproject.toml
        ├── hatch_build.py    # Custom build hook for platform wheels
        ├── src/
        │   └── injective_core/
        │       ├── __init__.py
        │       ├── cli.py
        │       └── bin/      # Binary directory (populated during build)
        ├── README.md
        └── LICENSE
```

## Prerequisites

Only **Docker** with Buildx support (Docker 20.10+) is required.

```bash
# Verify Docker and Buildx are available
docker buildx version
```

## NPM Package (`npm/injective-cli`)

### How It Works

The NPM package uses a **dual strategy** to deliver the correct binary:

1. **Optional Dependencies**: The base package declares platform-specific packages as `optionalDependencies`. NPM installs only the package matching the current platform (determined by `os` and `cpu` fields in package.json).

2. **Post-Install Fallback**: If optional dependencies are disabled (e.g., `--no-optional` flag), a post-install script downloads the appropriate binary from GitHub releases.

### Package Structure

- **Base Package**: `injective-cli` - Contains TypeScript wrapper code
- **Platform Packages**:
  - `injective-cli-darwin-arm64`
  - `injective-cli-linux-arm64`
  - `injective-cli-linux-x64`

### Installation

```bash
# Global installation
npm install -g injective-cli

# Or use with npx (no installation)
npx -p injective-cli injectived --help
```

### Key Files

- `src/index.ts`: Entry point that locates and executes the correct binary
- `src/install.ts`: Post-install script for fallback binary download

## PyPI Package (`pypi/injective-cli`)

### How It Works

The PyPI package uses **platform-specific wheels** (PEP 425):

1. Each wheel is tagged with a specific platform (e.g., `macosx_11_0_arm64`, `linux_x86_64`)
2. `pip` automatically downloads the wheel matching the current platform
3. The custom `hatch_build.py` hook includes the correct binary during wheel building

### Package Structure

- Source distribution (sdist) with Python wrapper code
- Platform wheels with embedded binaries:
  - `injective_core-1.17.2-py3-none-macosx_11_0_arm64.whl`
  - `injective_core-1.17.2-py3-none-linux_arm64.whl`
  - `injective_core-1.17.2-py3-none-linux_x86_64.whl`

### Installation

```bash
pip install injective-cli
```

### Key Files

- `src/injective_core/__init__.py`: Library functions (`get_binary_path()`, `run_binary()`)
- `src/injective_core/cli.py`: CLI entry point
- `hatch_build.py`: Custom hatchling build hook that selects binary based on `INJECTIVED_PLATFORM` environment variable

## Building Packages with Docker

### Prerequisites

Place pre-built binaries in `binaries/`:

```
binaries/
├── injectived-darwin-arm64
├── injectived-linux-arm64
├── injectived-linux-x64
├── libwasmvm.dylib
├── libwasmvm.aarch64.so
└── libwasmvm.x86_64.so
```

### Building All Packages

```bash
cd packaging
make all-build VERSION=1.17.2
```

Output will be in `output/`:

```
output/
├── npm/
│   ├── injective-cli/              # Base package
│   ├── injective-cli-darwin-arm64/ # Platform package
│   ├── injective-cli-linux-arm64/  # Platform package
│   └── injective-cli-linux-x64/    # Platform package
└── pypi/
    ├── injective_core-*.whl         # Platform wheels
    └── injective_core-*.tar.gz      # Source distribution
```

### Building Specific Package Types

```bash
# NPM only
make npm-build VERSION=1.17.2

# PyPI only  
make pypi-build VERSION=1.17.2
```

### Building Individual Platform Packages

```bash
# Single NPM platform package
make npm-build-darwin-arm64 VERSION=1.17.2

# Single PyPI wheel
make pypi-build-darwin-arm64 VERSION=1.17.2
```

### Using Docker Bake (Alternative)

```bash
# Build all NPM packages
docker buildx bake --file docker-bake.hcl npm-packages

# Build all PyPI packages  
docker buildx bake --file docker-bake.hcl pypi-packages

# Build everything
docker buildx bake --file docker-bake.hcl all-packages
```

## Publishing Packages

### NPM Publishing

**Important**: Platform packages MUST be published before the base package!

```bash
cd packaging

# 1. Build everything
make npm-build VERSION=1.17.2

# 2. Publish all NPM packages
make npm-publish NPM_TOKEN=xxx VERSION=1.17.2

# Or publish individual packages
make npm-publish-platform PLATFORM=darwin-arm64 NPM_TOKEN=xxx
```

### PyPI Publishing

```bash
cd packaging
make pypi-build VERSION=1.17.2
make pypi-publish PYPI_TOKEN=xxx VERSION=1.17.2
```

### TestPyPI (Testing)

```bash
cd packaging
make pypi-publish-test PYPI_TOKEN=xxx VERSION=1.17.2
```

## GitHub Actions Workflow

The `.github/workflows/publish-packages.yaml` workflow automates the entire process using Docker Buildx.

### Workflow Steps

1. **Build Binaries** (Matrix):
   - macOS ARM64 (macos-14 runner)
   - Linux ARM64 (cross-compilation with aarch64-linux-gnu-gcc)
   - Linux x64 (amd64)

2. **Build NPM Packages**:
   - Downloads all binaries
   - Uses Docker Buildx to build each platform package
   - Builds base package with TypeScript compilation

3. **Build PyPI Packages**:
   - Downloads all binaries
   - Uses Docker Buildx to build each platform wheel
   - Builds source distribution

4. **Publish NPM** (parallel):
   - Publishes platform-specific packages
   - Publishes base package

5. **Publish PyPI** (parallel):
   - Uploads all wheels and sdist to PyPI

### Triggering the Workflow

Via GitHub CLI:

```bash
gh workflow run publish-packages.yaml -f tag=v1.17.2
```

Via GitHub Web UI:

1. Go to Actions → "Publish NPM and PyPI Packages"
2. Click "Run workflow"
3. Enter tag (e.g., `v1.17.2`)

### Required Secrets

Configure these in GitHub repository settings:

- `NPM_TOKEN`: NPM authentication token with publish access
- `PYPI_API_TOKEN`: PyPI API token for uploading

## Platform Tag Reference

### NPM Platform Mapping

| Platform | NPM os | NPM cpu | Package Name |
|----------|--------|---------|--------------|
| macOS ARM64 | `darwin` | `arm64` | `injective-cli-darwin-arm64` |
| Linux ARM64 | `linux` | `arm64` | `injective-cli-linux-arm64` |
| Linux x64 | `linux` | `x64` | `injective-cli-linux-x64` |

### PyPI Platform Mapping

| Platform | Wheel Platform Tag | Binary Suffix |
|----------|-------------------|---------------|
| macOS ARM64 | `macosx_11_0_arm64` | `darwin-arm64` |
| Linux ARM64 | `linux_arm64` | `linux-arm64` |
| Linux x64 | `linux_x86_64` | `linux-x64` |

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make help` | Show available commands |
| `make check-docker` | Verify Docker buildx is available |
| `make npm-build` | Build all NPM packages |
| `make pypi-build` | Build all PyPI packages |
| `make all-build` | Build both NPM and PyPI packages |
| `make npm-publish` | Publish NPM packages |
| `make pypi-publish` | Publish PyPI packages |
| `make clean` | Clean all build artifacts |
| `make list-binaries` | List available binaries |

## Troubleshooting

### Docker: "Cannot connect to the Docker daemon"

Ensure Docker is running and your user has permissions:

```bash
sudo usermod -aG docker $USER
# Log out and log back in
```

### NPM: "Cannot find module"

If the base package can't find the binary:

```bash
# Check if platform package is installed
npm list -g injective-cli-darwin-arm64  # (adjust for your platform)

# Reinstall with optional dependencies
npm install -g injective-cli --no-save
```

### PyPI: "No matching distribution"

If pip can't find a wheel:

```bash
# Check available platforms
pip debug --verbose | grep compatible

# Install from source (fallback)
pip install --no-binary injective-cli injective-cli
```

### Build Issues

Clean and rebuild:

```bash
cd packaging && make clean
```

### Cross-Compilation Notes

For Linux ARM64 builds on x64 runners, the GitHub Actions workflow uses `aarch64-linux-gnu-gcc`:

```bash
CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOARCH=arm64 go build ...
```

## References

- [NPM Platform Documentation](https://docs.npmjs.com/cli/v10/configuring-npm/package-json#os)
- [Python Wheel Platform Tags (PEP 425)](https://packaging.python.org/specifications/binary-distribution-format/)
- [Hatchling Build System](https://hatch.pypa.io/latest/)
- [Publishing Binaries on NPM - Sentry Engineering](https://sentry.engineering/blog/publishing-binaries-on-npm)
- [Packaging Rust for NPM](https://blog.orhun.dev/packaging-rust-for-npm/)
- [Docker Buildx Documentation](https://docs.docker.com/buildx/)
