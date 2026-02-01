"""Injective Core - Injective blockchain node binary wrapper."""

__version__ = "1.17.2"
__all__ = ["get_binary_path", "run_binary"]

import os
import platform
import sys
from pathlib import Path
from typing import Optional


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
    
    if not binary_path.exists():
        raise FileNotFoundError(
            f"Binary not found at {binary_path}. "
            f"This package may not support your platform ({sys.platform} {platform.machine()}). "
            f"Supported platforms: macOS (arm64), Linux (arm64, x86_64)"
        )
    
    return binary_path


def run_binary(args: Optional[list] = None) -> int:
    """
    Run the injectived binary with the given arguments.
    
    Args:
        args: List of arguments to pass to the binary
        
    Returns:
        The exit code from the binary
    """
    import subprocess
    
    binary_path = get_binary_path()
    
    # Make binary executable on Unix
    if sys.platform != "win32":
        os.chmod(binary_path, 0o755)
    
    cmd = [str(binary_path)] + (args or [])
    
    # Replace current process with the binary
    # This provides the most transparent experience
    os.execv(str(binary_path), cmd)
    
    # Should never reach here
    return 1
