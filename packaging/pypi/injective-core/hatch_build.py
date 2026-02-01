"""
Custom hatchling build hook for platform-specific binary inclusion.

This script is used to include the correct binary during wheel builds
based on the target platform.
"""

import os
import shutil
import sys
from pathlib import Path

from hatchling.builders.hooks.plugin.interface import BuildHookInterface


class InjectivedBuildHook(BuildHookInterface):
    """Build hook to include platform-specific injectived binary."""
    
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
        
        # Copy the binary
        dest_binary = bin_dir / binary_name
        shutil.copy2(source_binary, dest_binary)
        
        # Make executable on Unix
        if os.name != "nt":
            os.chmod(dest_binary, 0o755)
            link_path = bin_dir / "injectived"
            if link_path.exists() or link_path.is_symlink():
                link_path.unlink()
            os.symlink(dest_binary.name, link_path)

        self.app.display_info(f"Included binary for {target_platform}: {binary_name}")

        lib_name = self.PLATFORM_LIBS.get(target_platform)
        if lib_name:
            source_lib = self._find_source_asset(lib_name)
            if source_lib:
                dest_lib = bin_dir / lib_name
                shutil.copy2(source_lib, dest_lib)
                self.app.display_info(f"Included wasmvm library for {target_platform}: {lib_name}")
            else:
                self.app.display_warning(
                    f"Wasmvm library not found for platform {target_platform} ({lib_name}). "
                    "Make sure the library is available in dist/binaries/"
                )
        
        # Set the platform tag for the wheel
        build_data["tag"] = f"py3-none-{target_platform}"
        build_data["pure_python"] = False
    
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
    
    def _find_source_asset(self, asset_name: str) -> Path | None:
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
