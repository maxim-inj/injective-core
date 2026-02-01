# =============================================================================
# Dockerfile for building Injective Core distribution packages
# =============================================================================
# This Dockerfile builds NPM and PyPI packages in a containerized environment.
# Uses multi-stage builds to separate concerns and minimize final image size.
#
# Usage:
#   docker buildx build --target npm-packages --output type=local,dest=./output .
#   docker buildx build --target pypi-packages --output type=local,dest=./output .
# =============================================================================

# -----------------------------------------------------------------------------
# Base stage with common dependencies
# -----------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM node:20-slim AS base

# Install common tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
    curl \
    gettext-base \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# -----------------------------------------------------------------------------
# NPM Builder Stage
# -----------------------------------------------------------------------------
FROM base AS npm-builder

# Install TypeScript globally
RUN npm install -g typescript

# Copy NPM package source
COPY npm/ ./npm/

# Build the base package
WORKDIR /build/npm/injective-core
RUN npm ci 2>/dev/null || npm install
RUN npm run build

# Return to build root
WORKDIR /build

# -----------------------------------------------------------------------------
# NPM Package Assembler Stage
# Arguments:
#   PLATFORM: Target platform (darwin-arm64, linux-arm64, linux-x64)
#   BINARY_PATH: Path to the pre-built binary
# -----------------------------------------------------------------------------
FROM npm-builder AS npm-package-assembler

ARG PLATFORM
ARG NODE_OS
ARG NODE_ARCH
ARG NODE_VERSION

# Copy binaries from build context
# Note: Expected files must exist - build will fail if not found
COPY binaries/ /tmp/binaries/

# Create platform-specific package
RUN set -e; \
    case "${PLATFORM}" in \
      darwin-arm64) wasmvm_lib="libwasmvm.dylib" ;; \
      linux-arm64) wasmvm_lib="libwasmvm.aarch64.so" ;; \
      linux-x64) wasmvm_lib="libwasmvm.x86_64.so" ;; \
      *) echo "Unsupported platform: ${PLATFORM}" && exit 1 ;; \
    esac; \
    pkg_dir="/output/npm/injective-cli-${PLATFORM}/bin"; \
    mkdir -p "$pkg_dir"; \
    cp /tmp/binaries/injectived-${PLATFORM} "$pkg_dir/injectived-${PLATFORM}"; \
    cp /tmp/binaries/${wasmvm_lib} "$pkg_dir/${wasmvm_lib}"; \
    chmod +x "$pkg_dir/injectived-${PLATFORM}"; \
    ln -sf "injectived-${PLATFORM}" "$pkg_dir/injectived"; \
    echo "Created NPM package for ${PLATFORM} with ${wasmvm_lib}"

# Generate package.json from template
RUN export node_pkg="injective-cli-${NODE_OS}-${NODE_ARCH}" && \
    export node_os="${NODE_OS}" && \
    export node_arch="${NODE_ARCH}" && \
    export node_version="${NODE_VERSION}" && \
    envsubst < /build/npm/package.json.tmpl > /output/npm/injective-cli-${PLATFORM}/package.json

# Copy base package to output
RUN cp -r /build/npm/injective-core /output/npm/injective-cli && \
    rm -rf /output/npm/injective-cli/node_modules /output/npm/injective-cli/src

# -----------------------------------------------------------------------------
# NPM Output Stage
# This stage only contains the NPM packages for clean export
# -----------------------------------------------------------------------------
FROM scratch AS npm-output
COPY --from=npm-package-assembler /output/npm/ /npm/

# -----------------------------------------------------------------------------
# PyPI Builder Stage
# -----------------------------------------------------------------------------
FROM python:3.12-slim AS pypi-builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install Python build tools
RUN pip install --no-cache-dir build hatchling twine

WORKDIR /build

# Copy PyPI package source
COPY pypi/injective-core/ ./pypi/injective-core/

# Set working directory for builds
WORKDIR /build/pypi/injective-core

# -----------------------------------------------------------------------------
# PyPI Package Assembler Stage
# Arguments:
#   PLATFORM: Target platform (darwin-arm64, linux-arm64, linux-x64)
#   PYPI_PLATFORM: PyPI platform tag (macosx_11_0_arm64, linux_x86_64, etc.)
#   VERSION: Package version
# -----------------------------------------------------------------------------
FROM pypi-builder AS pypi-package-assembler

ARG PLATFORM
ARG PYPI_PLATFORM
ARG VERSION

# Copy binaries from build context
# Note: Expected files must exist - build will fail if not found
COPY binaries/ /tmp/binaries/

RUN set -e; \
    case "${PLATFORM}" in \
      darwin-arm64) wasmvm_lib="libwasmvm.dylib" ;; \
      linux-arm64) wasmvm_lib="libwasmvm.aarch64.so" ;; \
      linux-x64) wasmvm_lib="libwasmvm.x86_64.so" ;; \
      *) echo "Unsupported platform: ${PLATFORM}" && exit 1 ;; \
    esac; \
    mkdir -p src/injective_core/bin; \
    cp /tmp/binaries/injectived-${PLATFORM} src/injective_core/bin/injectived-${PLATFORM}; \
    cp /tmp/binaries/${wasmvm_lib} src/injective_core/bin/${wasmvm_lib}; \
    chmod +x src/injective_core/bin/injectived-${PLATFORM}; \
    ln -sf "injectived-${PLATFORM}" src/injective_core/bin/injectived; \
    echo "Prepared binary for ${PLATFORM} with ${wasmvm_lib}"

# Update version in files
RUN sed -i "s/version = \"[^\"]*\"/version = \"${VERSION}\"/g" pyproject.toml && \
    sed -i "s/__version__ = \"[^\"]*\"/__version__ = \"${VERSION}\"/g" src/injective_core/__init__.py

# Build wheel with specific platform
RUN INJECTIVED_PLATFORM="${PYPI_PLATFORM}" python -m build --wheel --no-isolation

# Rename wheel to have correct platform tag (hatchling doesn't always set this correctly)
RUN cd dist && \
    for whl in *.whl; do \
        if echo "$whl" | grep -q "py3-none-any"; then \
            newname=$(echo "$whl" | sed "s/py3-none-any/${PYPI_PLATFORM}/"); \
            mv "$whl" "$newname"; \
            echo "Renamed $whl to $newname"; \
        fi; \
    done

# Build sdist (only needed once, but we do it for each platform and let the caller pick one)
RUN rm -f src/injective_core/bin/injectived && \
    python -m build --sdist

# Copy output
RUN mkdir -p /output/pypi && \
    cp dist/*.whl dist/*.tar.gz /output/pypi/ 2>/dev/null || true

# -----------------------------------------------------------------------------
# PyPI Output Stage
# This stage only contains the PyPI packages for clean export
# -----------------------------------------------------------------------------
FROM scratch AS pypi-output
COPY --from=pypi-package-assembler /output/pypi/ /pypi/

# -----------------------------------------------------------------------------
# All NPM Packages Stage
# Combines all platform-specific NPM packages
# -----------------------------------------------------------------------------
FROM scratch AS npm-packages

COPY --from=npm-package-assembler-darwin-arm64 /output/npm/ /
COPY --from=npm-package-assembler-linux-arm64 /output/npm/ /
COPY --from=npm-package-assembler-linux-x64 /output/npm/ /

# Note: This stage is a placeholder - actual multi-platform builds use the 
# docker-bake.hcl file or build command with different targets

# -----------------------------------------------------------------------------
# All PyPI Packages Stage
# Combines all platform-specific PyPI packages
# -----------------------------------------------------------------------------
FROM scratch AS pypi-packages

COPY --from=pypi-package-assembler-darwin-arm64 /output/pypi/ /
COPY --from=pypi-package-assembler-linux-arm64 /output/pypi/ /
COPY --from=pypi-package-assembler-linux-x64 /output/pypi/ /

# Note: This stage is a placeholder - actual multi-platform builds use the 
# docker-bake.hcl file or build command with different targets

# -----------------------------------------------------------------------------
# Single NPM Package Stage (for direct builds)
# Usage: docker build --target npm-single --build-arg PLATFORM=linux-x64 ...
# -----------------------------------------------------------------------------
FROM npm-package-assembler AS npm-single

# The output is already in /output/npm/

# -----------------------------------------------------------------------------
# Single PyPI Package Stage (for direct builds)
# Usage: docker build --target pypi-single --build-arg PLATFORM=linux-x64 ...
# -----------------------------------------------------------------------------
FROM pypi-package-assembler AS pypi-single

# The output is already in /output/pypi/

# -----------------------------------------------------------------------------
# Combined Packages Stage
# Outputs both NPM and PyPI packages
# -----------------------------------------------------------------------------
FROM scratch AS all-packages

COPY --from=npm-single /output/npm/ /npm/
COPY --from=pypi-single /output/pypi/ /pypi/
