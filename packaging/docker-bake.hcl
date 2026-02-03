// =============================================================================
// Docker Buildx Bake Configuration for Injective Core Distribution Packages
// =============================================================================
// Usage:
//   docker buildx bake --file docker-bake.hcl npm-packages
//   docker buildx bake --file docker-bake.hcl pypi-packages
//   docker buildx bake --file docker-bake.hcl all-packages
//
// Environment Variables:
//   VERSION - Package version (default: 1.17.2)
//   OUTPUT_DIR - Output directory (default: ./output)
// =============================================================================

variable "VERSION" {
    default = "1.17.2"
}

variable "OUTPUT_DIR" {
    default = "./output"
}

// =============================================================================
// NPM Platform-Specific Packages
// =============================================================================

target "npm-darwin-arm64" {
    dockerfile = "Dockerfile"
    target = "npm-package-assembler"
    args = {
        PLATFORM = "darwin-arm64"
        NODE_OS = "darwin"
        NODE_ARCH = "arm64"
        NODE_VERSION = "${VERSION}"
    }
    platforms = ["linux/amd64"]  // Build on AMD64, target doesn't matter for this
    output = ["type=local,dest=${OUTPUT_DIR}/npm"]
    tags = ["injective-core/npm-darwin-arm64:${VERSION}"]
}

target "npm-linux-arm64" {
    dockerfile = "Dockerfile"
    target = "npm-package-assembler"
    args = {
        PLATFORM = "linux-arm64"
        NODE_OS = "linux"
        NODE_ARCH = "arm64"
        NODE_VERSION = "${VERSION}"
    }
    platforms = ["linux/amd64"]
    output = ["type=local,dest=${OUTPUT_DIR}/npm"]
    tags = ["injective-core/npm-linux-arm64:${VERSION}"]
}

target "npm-linux-x64" {
    dockerfile = "Dockerfile"
    target = "npm-package-assembler"
    args = {
        PLATFORM = "linux-x64"
        NODE_OS = "linux"
        NODE_ARCH = "x64"
        NODE_VERSION = "${VERSION}"
    }
    platforms = ["linux/amd64"]
    output = ["type=local,dest=${OUTPUT_DIR}/npm"]
    tags = ["injective-core/npm-linux-x64:${VERSION}"]
}

// =============================================================================
// PyPI Platform-Specific Packages
// =============================================================================

target "pypi-darwin-arm64" {
    dockerfile = "Dockerfile"
    target = "pypi-package-assembler"
    args = {
        PLATFORM = "darwin-arm64"
        PYPI_PLATFORM = "macosx_11_0_arm64"
        VERSION = "${VERSION}"
    }
    platforms = ["linux/amd64"]
    output = ["type=local,dest=${OUTPUT_DIR}/pypi"]
    tags = ["injective-core/pypi-darwin-arm64:${VERSION}"]
}

target "pypi-darwin-x64" {
    dockerfile = "Dockerfile"
    target = "pypi-package-assembler"
    args = {
        PLATFORM = "darwin-x64"
        PYPI_PLATFORM = "macosx_11_0_x86_64"
        VERSION = "${VERSION}"
    }
    platforms = ["linux/amd64"]
    output = ["type=local,dest=${OUTPUT_DIR}/pypi"]
    tags = ["injective-core/pypi-darwin-x64:${VERSION}"]
}

target "pypi-linux-arm64" {
    dockerfile = "Dockerfile"
    target = "pypi-package-assembler"
    args = {
        PLATFORM = "linux-arm64"
        PYPI_PLATFORM = "manylinux_2_17_aarch64"
        VERSION = "${VERSION}"
    }
    platforms = ["linux/amd64"]
    output = ["type=local,dest=${OUTPUT_DIR}/pypi"]
    tags = ["injective-core/pypi-linux-arm64:${VERSION}"]
}

target "pypi-linux-x64" {
    dockerfile = "Dockerfile"
    target = "pypi-package-assembler"
    args = {
        PLATFORM = "linux-x64"
        PYPI_PLATFORM = "manylinux_2_17_x86_64"
        VERSION = "${VERSION}"
    }
    platforms = ["linux/amd64"]
    output = ["type=local,dest=${OUTPUT_DIR}/pypi"]
    tags = ["injective-core/pypi-linux-x64:${VERSION}"]
}

// =============================================================================
// Group Targets
// =============================================================================

group "npm-packages" {
    targets = ["npm-darwin-arm64", "npm-linux-arm64", "npm-linux-x64"]
}

group "pypi-packages" {
    targets = ["pypi-darwin-arm64", "pypi-linux-arm64", "pypi-linux-x64"]
}

group "all-packages" {
    targets = ["npm-packages", "pypi-packages"]
}

// =============================================================================
// Default Target
// =============================================================================

group "default" {
    targets = ["all-packages"]
}
