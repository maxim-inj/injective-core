"""
Custom hatchling build hook for platform-specific binary inclusion.

This script is used to include the correct binary during wheel builds
based on the target platform.
"""

import os
import tarfile
import tempfile
from pathlib import Path

import zstandard as zstd

from hatchling.builders.hooks.plugin.interface import BuildHookInterface
from typing import Optional


class InjectivedBuildHook(BuildHookInterface):
    """Build hook to include platform-specific injectived binary."""

    PAYLOAD_ARCHIVE = "injectived.tar.zst"
    ZSTD_LEVEL = 20
    
    # Mapping of Python platform tags to binary names
    PLATFORM_BINARIES = {
        # macOS
        "macosx_11_0_arm64": "injectived-darwin-arm64",
        "macosx_12_0_arm64": "injectived-darwin-arm64",
        "macosx_13_0_arm64": "injectived-darwin-arm64",
        "macosx_14_0_arm64": "injectived-darwin-arm64",
        "macosx_11_0_x86_64": "injectived-darwin-x64",
        "macosx_12_0_x86_64": "injectived-darwin-x64",
        "macosx_13_0_x86_64": "injectived-darwin-x64",
        "macosx_14_0_x86_64": "injectived-darwin-x64",
        # Linux
        "manylinux2014_x86_64": "injectived-linux-x64",
        "manylinux_2_17_x86_64": "injectived-linux-x64",
        "manylinux_2_28_x86_64": "injectived-linux-x64",
        "manylinux2014_aarch64": "injectived-linux-arm64",
        "manylinux_2_17_aarch64": "injectived-linux-arm64",
        "manylinux_2_28_aarch64": "injectived-linux-arm64",
        "linux_x86_64": "injectived-linux-x64",
        "linux_aarch64": "injectived-linux-arm64",
        "linux_arm64": "injectived-linux-arm64",
    }

    PLATFORM_LIBS = {
        # macOS
        "macosx_11_0_arm64": "libwasmvm.dylib",
        "macosx_12_0_arm64": "libwasmvm.dylib",
        "macosx_13_0_arm64": "libwasmvm.dylib",
        "macosx_14_0_arm64": "libwasmvm.dylib",
        "macosx_11_0_x86_64": "libwasmvm.dylib",
        "macosx_12_0_x86_64": "libwasmvm.dylib",
        "macosx_13_0_x86_64": "libwasmvm.dylib",
        "macosx_14_0_x86_64": "libwasmvm.dylib",
        # Linux
        "manylinux2014_x86_64": "libwasmvm.x86_64.so",
        "manylinux_2_17_x86_64": "libwasmvm.x86_64.so",
        "manylinux_2_28_x86_64": "libwasmvm.x86_64.so",
        "manylinux2014_aarch64": "libwasmvm.aarch64.so",
        "manylinux_2_17_aarch64": "libwasmvm.aarch64.so",
        "manylinux_2_28_aarch64": "libwasmvm.aarch64.so",
        "linux_x86_64": "libwasmvm.x86_64.so",
        "linux_aarch64": "libwasmvm.aarch64.so",
        "linux_arm64": "libwasmvm.aarch64.so",
    }
    
    def initialize(self, version, build_data):
        """
        Initialize the build hook.
        
        Copies the appropriate binary into the package based on the target platform.
        """
        # Get the target platform from environment variable or build data
        target_platform = os.environ.get("INJECTIVED_PLATFORM", "")
        
        # If no target platform specified, try to detect from current platform
        if not target_platform:
            target_platform = self._detect_current_platform()
        
        if not target_platform:
            self.app.display_warning(
                "No target platform specified and could not detect current platform. "
                "Binary will not be included. "
                "Set INJECTIVED_PLATFORM environment variable to specify the target."
            )
            return
        
        # Get the binary name for this platform
        binary_name = self.PLATFORM_BINARIES.get(target_platform)
        if not binary_name:
            self.app.display_warning(
                f"Unknown platform: {target_platform}. "
                f"Supported platforms: {', '.join(sorted(set(self.PLATFORM_BINARIES.values())))}"
            )
            return
        
        # Source binary path (from dist/binaries or similar)
        source_binary = self._find_source_asset(binary_name)
        if not source_binary:
            self.app.display_warning(
                f"Binary not found for platform {target_platform} ({binary_name}). "
                "Make sure the binary is built and available in dist/binaries/"
            )
            return
        
        # Destination path in the package
        package_dir = Path(self.root) / "src" / "injective_core"
        bin_dir = package_dir / "bin"
        bin_dir.mkdir(parents=True, exist_ok=True)

        lib_name = self.PLATFORM_LIBS.get(target_platform)
        source_lib = None
        if lib_name:
            source_lib = self._find_source_asset(lib_name)
            if not source_lib:
                self.app.display_warning(
                    f"Wasmvm library not found for platform {target_platform} ({lib_name}). "
                    "Make sure the library is available in dist/binaries/"
                )

        self._build_payload(
            bin_dir=bin_dir,
            binary_name=binary_name,
            source_binary=source_binary,
            lib_name=lib_name,
            source_lib=source_lib,
        )

        self.app.display_info(f"Included compressed payload for {target_platform}: {binary_name}")
        
        # Set the platform tag for the wheel
        build_data["tag"] = f"py3-none-{target_platform}"
        build_data["pure_python"] = False

    def _build_payload(
        self,
        bin_dir: Path,
        binary_name: str,
        source_binary: Path,
        lib_name: Optional[str],
        source_lib: Optional[Path],
    ) -> None:
        tmp_tar_path = None
        try:
            with tempfile.NamedTemporaryFile(delete=False) as tmp_tar:
                tmp_tar_path = Path(tmp_tar.name)
            with tarfile.open(tmp_tar_path, "w") as tar:
                tar.add(source_binary, arcname=binary_name)
                if source_lib and lib_name:
                    tar.add(source_lib, arcname=lib_name)

            params = zstd.ZstdCompressionParameters.from_level(
                self.ZSTD_LEVEL,
                enable_ldm=True,
                window_log=27,
            )
            compressor = zstd.ZstdCompressor(compression_params=params)
            archive_path = bin_dir / self.PAYLOAD_ARCHIVE
            with tmp_tar_path.open("rb") as src, archive_path.open("wb") as dst:
                compressor.copy_stream(src, dst)
        finally:
            if tmp_tar_path and tmp_tar_path.exists():
                tmp_tar_path.unlink()
    
    def _detect_current_platform(self) -> str:
        """Detect the current platform."""
        import platform
        
        system = platform.system().lower()
        machine = platform.machine().lower()
        
        if system == "darwin":
            if machine == "arm64":
                return "macosx_11_0_arm64"
            elif machine in ("x86_64", "amd64"):
                return "macosx_11_0_x86_64"
        elif system == "linux":
            if machine in ("arm64", "aarch64"):
                return "linux_arm64"
            elif machine in ("x86_64", "amd64"):
                return "linux_x86_64"
        
        return ""
    
    def _find_source_asset(self, asset_name: str) -> Optional[Path]:
        """Find the source asset in the expected locations."""
        search_paths = [
            Path(self.root) / ".." / ".." / "binaries" / asset_name,
            Path(self.root) / "binaries" / asset_name,
            Path(self.root) / ".." / "binaries" / asset_name,
        ]
        
        for path in search_paths:
            if path.exists():
                return path
        
        return None
