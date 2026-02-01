# Injective CLI

Python package wrapper for the Injective blockchain node binary (`injectived`).

## Installation

```bash
pip install injective-cli
```

Or use with uvx (no installation required):

```bash
uvx --from injective-cli injectived --help
```

## Supported Platforms

- macOS ARM64 (Apple Silicon)
- Linux ARM64
- Linux x64

## Using injectived

After installation, the `injectived` command is available:

```bash
injectived --help
injectived init
injectived start
```

### Python API

You can also use the package programmatically:

```python
from injective_core import get_binary_path, run_binary

# Get the path to the binary
binary_path = get_binary_path()
print(f"Binary located at: {binary_path}")

# Run the binary with arguments
run_binary(["--help"])
```

Note: the import path remains `injective_core` even though the package name is `injective-cli`.

## How It Works

This package bundles platform-specific binaries as wheel distributions. When you install from PyPI, pip will download the correct wheel for your platform. Each wheel contains the appropriate `injectived` binary for the target platform.

## Building from Source

To build platform-specific wheels:

```bash
# Set the target platform
export INJECTIVED_PLATFORM=linux_x86_64  # or linux_arm64, darwin_arm64, darwin_x86_64

# Build the wheel
pip install build
python -m build --wheel
```

## License

BSL-1.1
