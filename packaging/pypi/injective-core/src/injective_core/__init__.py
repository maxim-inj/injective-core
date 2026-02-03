"""Injective Core - Injective blockchain node binary wrapper."""

__version__ = "1.17.2.post2"
__all__ = ["get_binary_path", "run_binary"]

import os
import platform
import sys
import tarfile
import tempfile
import shutil
from pathlib import Path
from typing import Optional

import zstandard as zstd

WASMVM_LIB_NAMES = (
    "libwasmvm.x86_64.so",
    "libwasmvm.aarch64.so",
    "libwasmvm.dylib",
)

PAYLOAD_ARCHIVE = "injectived.tar.zst"


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


def _normalize_arch() -> str:
    arch = platform.machine().lower()
    if arch in ("x86_64", "amd64"):
        return "x64"
    if arch in ("aarch64", "arm64"):
        return "arm64"
    return arch


def _platform_key() -> str:
    return f"{sys.platform}-{_normalize_arch()}"


def _is_within_directory(directory: Path, target: Path) -> bool:
    directory_abs = directory.resolve()
    target_abs = target.resolve()
    return os.path.commonpath([str(directory_abs)]) == os.path.commonpath(
        [str(directory_abs), str(target_abs)]
    )


def _safe_extract(tar: tarfile.TarFile, path: Path) -> None:
    for member in tar.getmembers():
        member_path = path / member.name
        if not _is_within_directory(path, member_path):
            raise RuntimeError(f"Blocked unsafe tar entry: {member.name}")
    tar.extractall(path)


def _ensure_payload_unpacked(bin_dir: Path) -> None:
    archive_path = bin_dir / PAYLOAD_ARCHIVE
    if not archive_path.exists():
        return

    binary_name = get_binary_name()
    binary_path = bin_dir / binary_name
    platform_binary = bin_dir / f"injectived-{_platform_key()}"

    if binary_path.exists() or platform_binary.exists():
        return

    dctx = zstd.ZstdDecompressor()

    tmp_tar_path = None
    try:
        with tempfile.NamedTemporaryFile(delete=False) as tmp_tar:
            tmp_tar_path = Path(tmp_tar.name)
            with archive_path.open("rb") as archive_file:
                with dctx.stream_reader(archive_file) as reader:
                    shutil.copyfileobj(reader, tmp_tar)

        with tarfile.open(tmp_tar_path, "r:") as tar:
            _safe_extract(tar, bin_dir)
    finally:
        if tmp_tar_path and tmp_tar_path.exists():
            tmp_tar_path.unlink()

    if platform_binary.exists() and not binary_path.exists():
        try:
            os.symlink(platform_binary.name, binary_path)
        except OSError:
            shutil.copyfile(platform_binary, binary_path)

    if os.name != "nt":
        if platform_binary.exists():
            os.chmod(platform_binary, 0o755)
        if binary_path.exists():
            os.chmod(binary_path, 0o755)


def get_binary_path() -> Path:
    """
    Get the path to the injectived binary.
    
    The binary is bundled with the package in the 'bin' directory.
    """
    # Get the directory where this module is located
    module_dir = Path(__file__).parent.resolve()
    bin_dir = module_dir / "bin"
    _ensure_payload_unpacked(bin_dir)
    binary_path = bin_dir / get_binary_name()
    if binary_path.exists():
        return binary_path

    fallback_path = bin_dir / f"injectived-{_platform_key()}"
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
