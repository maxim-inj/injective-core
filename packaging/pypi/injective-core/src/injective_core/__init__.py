"""Injective Core - Injective blockchain node binary wrapper."""

__version__ = "1.17.2"
__all__ = ["get_binary_path", "run_binary"]

import os
import platform
import sys
from pathlib import Path
from typing import Optional

WASMVM_LIB_NAMES = (
    "libwasmvm.x86_64.so",
    "libwasmvm.aarch64.so",
    "libwasmvm.dylib",
)


def _has_wasmvm_lib(bin_dir: Path) -> bool:
    return any((bin_dir / name).exists() for name in WASMVM_LIB_NAMES)


def _prepend_env_path(key: str, value: str) -> None:
    current = os.environ.get(key)
    os.environ[key] = f"{value}{os.pathsep}{current}" if current else value


def _ensure_wasmvm_library_path(bin_dir: Path) -> None:
    if not _has_wasmvm_lib(bin_dir):
        return

    if sys.platform.startswith("linux"):
        _prepend_env_path("LD_LIBRARY_PATH", str(bin_dir))
    elif sys.platform == "darwin":
        _prepend_env_path("DYLD_LIBRARY_PATH", str(bin_dir))
        _prepend_env_path("DYLD_FALLBACK_LIBRARY_PATH", str(bin_dir))


def get_binary_name() -> str:
    """Get the binary name based on the platform."""
    if sys.platform == "win32":
        return "injectived.exe"
    return "injectived"


def get_binary_path() -> Path:
    """
    Get the path to the injectived binary.
    
    The binary is bundled with the package in the 'bin' directory.
    """
    # Get the directory where this module is located
    module_dir = Path(__file__).parent.resolve()
    bin_dir = module_dir / "bin"
    binary_path = bin_dir / get_binary_name()
    if binary_path.exists():
        return binary_path

    arch = platform.machine().lower()
    if arch in ("x86_64", "amd64"):
        arch = "x64"
    elif arch in ("aarch64", "arm64"):
        arch = "arm64"

    platform_key = f"{sys.platform}-{arch}"
    fallback_path = bin_dir / f"injectived-{platform_key}"
    if fallback_path.exists():
        return fallback_path

    raise FileNotFoundError(
        f"Binary not found at {binary_path} or {fallback_path}. "
        f"This package may not support your platform ({sys.platform} {platform.machine()}). "
        f"Supported platforms: macOS (arm64), Linux (arm64, x86_64)"
    )


def run_binary(args: Optional[list] = None) -> int:
    """
    Run the injectived binary with the given arguments.
    
    Args:
        args: List of arguments to pass to the binary
        
    Returns:
        The exit code from the binary
    """
    binary_path = get_binary_path()
    bin_dir = binary_path.parent
    _ensure_wasmvm_library_path(bin_dir)
    
    # Make binary executable on Unix
    if sys.platform != "win32":
        os.chmod(binary_path, 0o755)
    
    cmd = [str(binary_path)] + (args or [])
    
    # Replace current process with the binary
    # This provides the most transparent experience
    os.execv(str(binary_path), cmd)
    
    # Should never reach here
    return 1
