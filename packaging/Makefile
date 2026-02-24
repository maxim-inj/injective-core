# =============================================================================
# Distribution Makefile for Injective Core (Docker Buildx Edition)
# =============================================================================
# This Makefile uses Docker Buildx to build NPM and PyPI packages in isolated
# containers. No local tool installation is required - only Docker.
#
# Prerequisites:
#   - Docker with buildx support (Docker 20.10+)
#
# Usage:
#   make npm-build VERSION=1.17.2-build3    # Build all NPM packages
#   make pypi-build VERSION=1.17.2-build3 PYPI_VERSION=1.17.2.post3   # Build all PyPI packages
#   make all-build VERSION=1.17.2-build3 PYPI_VERSION=1.17.2.post3    # Build all packages
#
# Publishing (requires registry credentials):
#   make npm-publish NPM_TOKEN=xxx VERSION=1.17.2-build3 NPM_TAG=build
#   make pypi-publish PYPI_TOKEN=xxx VERSION=1.17.2-build3 PYPI_VERSION=1.17.2.post3
# =============================================================================

# Configuration
VERSION ?= 1.18.0
PYPI_VERSION ?= 1.18.0
NPM_VERSION ?= $(shell printf "%s" "$(VERSION)" | sed -E 's/^([0-9]+\\.[0-9]+\\.[0-9]+)\\.post([0-9]+)$$/\\1-build\\2/; s/^([0-9]+\\.[0-9]+\\.[0-9]+)-post\\.([0-9]+)$$/\\1-build\\2/')
NPM_TAG ?= build
OUTPUT_DIR ?= $(CURDIR)/output
BINARIES_DIR ?= $(CURDIR)/binaries

# Docker configuration
BUILDER_NAME ?= injective-core-builder
BAKE_FILE ?= docker-bake.hcl
DOCKERFILE ?= Dockerfile

# =============================================================================
# Helper Targets
# =============================================================================

.PHONY: help check-docker ensure-builder

help:
	@echo "Injective Core Distribution - Docker Buildx Makefile"
	@echo ""
	@echo "Build Targets:"
	@echo "  npm-build        - Build all NPM packages (platform-specific + base)"
	@echo "  pypi-build       - Build all PyPI packages (wheels + sdist)"
	@echo "  all-build        - Build both NPM and PyPI packages"
	@echo ""
	@echo "Publish Targets:"
	@echo "  npm-publish      - Publish NPM packages to registry"
	@echo "  pypi-publish     - Publish PyPI packages to registry"
	@echo "  all-publish      - Publish both NPM and PyPI packages"
	@echo ""
	@echo "Utility Targets:"
	@echo "  clean            - Remove all build artifacts"
	@echo "  clean-output     - Remove output directory"
	@echo "  check-docker     - Verify Docker buildx is available"
	@echo "  ensure-builder   - Create/use docker buildx builder"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo "  PYPI_VERSION=$(PYPI_VERSION)"
	@echo "  NPM_TAG=$(NPM_TAG)"
	@echo "  NPM_VERSION=$(NPM_VERSION)"
	@echo "  OUTPUT_DIR=$(OUTPUT_DIR)"
	@echo "  BINARIES_DIR=$(BINARIES_DIR)"

check-docker:
	@which docker > /dev/null || (echo "Error: Docker not found" && exit 1)
	@docker buildx version > /dev/null 2>&1 || (echo "Error: Docker buildx not available" && exit 1)
	@echo "Docker buildx is available"

ensure-builder: check-docker
	@docker buildx inspect $(BUILDER_NAME) > /dev/null 2>&1 || \
		(docker buildx create --name $(BUILDER_NAME) --use && \
		 echo "Created buildx builder: $(BUILDER_NAME)") || \
		echo "Using existing buildx builder: $(BUILDER_NAME)"

# =============================================================================
# NPM Package Targets
# =============================================================================

.PHONY: npm-build npm-build-base npm-build-platforms npm-publish npm-publish-platform npm-publish-base

# Build all NPM packages
npm-build: npm-build-base npm-build-platforms
	@echo "NPM packages built successfully in $(OUTPUT_DIR)/npm/"

# Build base NPM package (TypeScript compilation)
npm-build-base: ensure-builder check-binaries
	@echo "Building NPM base package..."
	@mkdir -p $(OUTPUT_DIR)/npm
	@docker buildx build \
		--file $(DOCKERFILE) \
		--target npm-builder \
		--output type=local,dest=$(OUTPUT_DIR)/npm-temp \
		--build-arg VERSION=$(VERSION) \
		.
	@# Copy the base package from the build output
	@cp -r $(OUTPUT_DIR)/npm-temp/build/npm/injective-core $(OUTPUT_DIR)/npm/injective-core 2>/dev/null || \
		cp -r $(OUTPUT_DIR)/npm-temp/npm/injective-core $(OUTPUT_DIR)/npm/injective-core 2>/dev/null || \
		echo "Note: Base package may be built with platform packages"
	@rm -rf $(OUTPUT_DIR)/npm-temp
	@echo "Base NPM package built"

# Build all platform-specific NPM packages (only for available binaries)
npm-build-platforms: npm-build-darwin-arm64 npm-build-linux-arm64 npm-build-linux-x64

npm-build-darwin-arm64: ensure-builder check-binaries
	@echo "Building NPM package for darwin-arm64..."
	@mkdir -p $(OUTPUT_DIR)/npm
	@docker buildx build \
		--file $(DOCKERFILE) \
		--target npm-output \
		--output type=local,dest=$(OUTPUT_DIR) \
		--build-arg PLATFORM=darwin-arm64 \
		--build-arg NODE_OS=darwin \
		--build-arg NODE_ARCH=arm64 \
		--build-arg NODE_VERSION=$(NPM_VERSION) \
		.

npm-build-linux-arm64: ensure-builder check-binaries
	@echo "Building NPM package for linux-arm64..."
	@mkdir -p $(OUTPUT_DIR)/npm
	@docker buildx build \
		--file $(DOCKERFILE) \
		--target npm-output \
		--output type=local,dest=$(OUTPUT_DIR) \
		--build-arg PLATFORM=linux-arm64 \
		--build-arg NODE_OS=linux \
		--build-arg NODE_ARCH=arm64 \
		--build-arg NODE_VERSION=$(NPM_VERSION) \
		.

npm-build-linux-x64: ensure-builder check-binaries
	@echo "Building NPM package for linux-x64..."
	@mkdir -p $(OUTPUT_DIR)/npm
	@docker buildx build \
		--file $(DOCKERFILE) \
		--target npm-output \
		--output type=local,dest=$(OUTPUT_DIR) \
		--build-arg PLATFORM=linux-x64 \
		--build-arg NODE_OS=linux \
		--build-arg NODE_ARCH=x64 \
		--build-arg NODE_VERSION=$(NPM_VERSION) \
		.

# Build NPM packages using docker-bake (alternative)
npm-build-bake: ensure-builder check-binaries
	@echo "Building NPM packages with docker-bake..."
	@VERSION=$(VERSION) OUTPUT_DIR=$(OUTPUT_DIR) \
		docker buildx bake --file $(BAKE_FILE) npm-packages

# Publish NPM packages
npm-publish: npm-build npm-publish-platforms npm-publish-base

npm-publish-platforms: npm-publish-darwin-arm64 npm-publish-linux-arm64 npm-publish-linux-x64

npm-publish-darwin-arm64:
	@echo "Publishing NPM package for darwin-arm64..."
	@if [ -z "$(NPM_TOKEN)" ]; then \
		echo "Error: NPM_TOKEN is required for publishing"; \
		exit 1; \
	fi
	@mkdir -p $(OUTPUT_DIR)
	@printf "//registry.npmjs.org/:_authToken=%s\n" "$(NPM_TOKEN)" > $(OUTPUT_DIR)/.npmrc
	@cd $(OUTPUT_DIR)/npm/injective-core-darwin-arm64 && \
		VERSION=$$(node -p "require('./package.json').version"); \
		TAG_ARGS=""; \
		TAG="$(NPM_TAG)"; \
		if [ -z "$$TAG" ] && echo "$$VERSION" | grep -q '-'; then \
			if echo "$$VERSION" | grep -Eq '^[0-9]+\\.[0-9]+\\.[0-9]+-build\\.[0-9]+$$'; then \
				TAG=$$(echo "$$VERSION" | sed -E 's/^([0-9]+\\.[0-9]+\\.[0-9]+)-build\\.([0-9]+)$$/\\1.post\\2/'); \
			else \
				case "$$VERSION" in \
					*-rc.*) TAG="rc" ;; \
					*-beta.*|*-b.*) TAG="beta" ;; \
					*-alpha.*|*-a.*) TAG="alpha" ;; \
					*-dev.*) TAG="dev" ;; \
					*) TAG="next" ;; \
				esac; \
			fi; \
		fi; \
		if [ -n "$$TAG" ]; then TAG_ARGS="--tag $$TAG"; fi; \
		NPM_CONFIG_USERCONFIG=$(OUTPUT_DIR)/.npmrc npm publish --access public $$TAG_ARGS --registry https://registry.npmjs.org/
	@rm -f $(OUTPUT_DIR)/.npmrc

npm-publish-linux-arm64:
	@echo "Publishing NPM package for linux-arm64..."
	@if [ -z "$(NPM_TOKEN)" ]; then \
		echo "Error: NPM_TOKEN is required for publishing"; \
		exit 1; \
	fi
	@mkdir -p $(OUTPUT_DIR)
	@printf "//registry.npmjs.org/:_authToken=%s\n" "$(NPM_TOKEN)" > $(OUTPUT_DIR)/.npmrc
	@cd $(OUTPUT_DIR)/npm/injective-core-linux-arm64 && \
		VERSION=$$(node -p "require('./package.json').version"); \
		TAG_ARGS=""; \
		TAG="$(NPM_TAG)"; \
		if [ -z "$$TAG" ] && echo "$$VERSION" | grep -q '-'; then \
			if echo "$$VERSION" | grep -Eq '^[0-9]+\\.[0-9]+\\.[0-9]+-build\\.[0-9]+$$'; then \
				TAG=$$(echo "$$VERSION" | sed -E 's/^([0-9]+\\.[0-9]+\\.[0-9]+)-build\\.([0-9]+)$$/\\1.post\\2/'); \
			else \
				case "$$VERSION" in \
					*-rc.*) TAG="rc" ;; \
					*-beta.*|*-b.*) TAG="beta" ;; \
					*-alpha.*|*-a.*) TAG="alpha" ;; \
					*-dev.*) TAG="dev" ;; \
					*) TAG="next" ;; \
				esac; \
			fi; \
		fi; \
		if [ -n "$$TAG" ]; then TAG_ARGS="--tag $$TAG"; fi; \
		NPM_CONFIG_USERCONFIG=$(OUTPUT_DIR)/.npmrc npm publish --access public $$TAG_ARGS --registry https://registry.npmjs.org/
	@rm -f $(OUTPUT_DIR)/.npmrc

npm-publish-linux-x64:
	@echo "Publishing NPM package for linux-x64..."
	@if [ -z "$(NPM_TOKEN)" ]; then \
		echo "Error: NPM_TOKEN is required for publishing"; \
		exit 1; \
	fi
	@mkdir -p $(OUTPUT_DIR)
	@printf "//registry.npmjs.org/:_authToken=%s\n" "$(NPM_TOKEN)" > $(OUTPUT_DIR)/.npmrc
	@cd $(OUTPUT_DIR)/npm/injective-core-linux-x64 && \
		VERSION=$$(node -p "require('./package.json').version"); \
		TAG_ARGS=""; \
		TAG="$(NPM_TAG)"; \
		if [ -z "$$TAG" ] && echo "$$VERSION" | grep -q '-'; then \
			if echo "$$VERSION" | grep -Eq '^[0-9]+\\.[0-9]+\\.[0-9]+-build\\.[0-9]+$$'; then \
				TAG=$$(echo "$$VERSION" | sed -E 's/^([0-9]+\\.[0-9]+\\.[0-9]+)-build\\.([0-9]+)$$/\\1.post\\2/'); \
			else \
				case "$$VERSION" in \
					*-rc.*) TAG="rc" ;; \
					*-beta.*|*-b.*) TAG="beta" ;; \
					*-alpha.*|*-a.*) TAG="alpha" ;; \
					*-dev.*) TAG="dev" ;; \
					*) TAG="next" ;; \
				esac; \
			fi; \
		fi; \
		if [ -n "$$TAG" ]; then TAG_ARGS="--tag $$TAG"; fi; \
		NPM_CONFIG_USERCONFIG=$(OUTPUT_DIR)/.npmrc npm publish --access public $$TAG_ARGS --registry https://registry.npmjs.org/
	@rm -f $(OUTPUT_DIR)/.npmrc

npm-publish-base:
	@echo "Publishing NPM base package..."
	@if [ -z "$(NPM_TOKEN)" ]; then \
		echo "Error: NPM_TOKEN is required for publishing"; \
		exit 1; \
	fi
	@sleep 5  # Wait for platform packages to be available
	@mkdir -p $(OUTPUT_DIR)
	@printf "//registry.npmjs.org/:_authToken=%s\n" "$(NPM_TOKEN)" > $(OUTPUT_DIR)/.npmrc
	@cd $(OUTPUT_DIR)/npm/injective-core && \
		VERSION=$$(node -p "require('./package.json').version"); \
		TAG_ARGS=""; \
		TAG="$(NPM_TAG)"; \
		if [ -z "$$TAG" ] && echo "$$VERSION" | grep -q '-'; then \
			if echo "$$VERSION" | grep -Eq '^[0-9]+\\.[0-9]+\\.[0-9]+-build\\.[0-9]+$$'; then \
				TAG=$$(echo "$$VERSION" | sed -E 's/^([0-9]+\\.[0-9]+\\.[0-9]+)-build\\.([0-9]+)$$/\\1.post\\2/'); \
			else \
				case "$$VERSION" in \
					*-rc.*) TAG="rc" ;; \
					*-beta.*|*-b.*) TAG="beta" ;; \
					*-alpha.*|*-a.*) TAG="alpha" ;; \
					*-dev.*) TAG="dev" ;; \
					*) TAG="next" ;; \
				esac; \
			fi; \
		fi; \
		if [ -n "$$TAG" ]; then TAG_ARGS="--tag $$TAG"; fi; \
		NPM_CONFIG_USERCONFIG=$(OUTPUT_DIR)/.npmrc npm publish --access public $$TAG_ARGS --registry https://registry.npmjs.org/
	@rm -f $(OUTPUT_DIR)/.npmrc

# =============================================================================
# PyPI Package Targets
# =============================================================================

.PHONY: pypi-build pypi-build-wheels pypi-publish pypi-publish-test

# Build all PyPI packages
pypi-build: pypi-build-wheels
	@echo "PyPI packages built successfully in $(OUTPUT_DIR)/pypi/"

# Build all platform-specific wheels (only for available binaries)
pypi-build-wheels: pypi-build-darwin-arm64 pypi-build-linux-arm64 pypi-build-linux-x64 pypi-build-sdist

pypi-build-darwin-arm64: ensure-builder check-binaries
	@echo "Building PyPI wheel for darwin-arm64..."
	@mkdir -p $(OUTPUT_DIR)/pypi
	@docker buildx build \
		--file $(DOCKERFILE) \
		--target pypi-output \
		--output type=local,dest=$(OUTPUT_DIR) \
		--build-arg PLATFORM=darwin-arm64 \
		--build-arg PYPI_PLATFORM=macosx_11_0_arm64 \
		--build-arg VERSION=$(PYPI_VERSION) \
		.

pypi-build-darwin-x64: ensure-builder check-binaries
	@echo "Building PyPI wheel for darwin-x64..."
	@mkdir -p $(OUTPUT_DIR)/pypi
	@docker buildx build \
		--file $(DOCKERFILE) \
		--target pypi-output \
		--output type=local,dest=$(OUTPUT_DIR) \
		--build-arg PLATFORM=darwin-x64 \
		--build-arg PYPI_PLATFORM=macosx_11_0_x86_64 \
		--build-arg VERSION=$(PYPI_VERSION) \
		.

pypi-build-linux-arm64: ensure-builder check-binaries
	@echo "Building PyPI wheel for linux-arm64..."
	@mkdir -p $(OUTPUT_DIR)/pypi
	@docker buildx build \
		--file $(DOCKERFILE) \
		--target pypi-output \
		--output type=local,dest=$(OUTPUT_DIR) \
		--build-arg PLATFORM=linux-arm64 \
		--build-arg PYPI_PLATFORM=manylinux_2_17_aarch64 \
		--build-arg VERSION=$(PYPI_VERSION) \
		.

pypi-build-linux-x64: ensure-builder check-binaries
	@echo "Building PyPI wheel for linux-x64..."
	@mkdir -p $(OUTPUT_DIR)/pypi
	@docker buildx build \
		--file $(DOCKERFILE) \
		--target pypi-output \
		--output type=local,dest=$(OUTPUT_DIR) \
		--build-arg PLATFORM=linux-x64 \
		--build-arg PYPI_PLATFORM=manylinux_2_17_x86_64 \
		--build-arg VERSION=$(PYPI_VERSION) \
		.

pypi-build-sdist: ensure-builder
	@echo "Building PyPI source distribution..."
	@mkdir -p $(OUTPUT_DIR)/pypi
	@echo "Note: Source distribution is built with platform wheels"

# Build PyPI packages using docker-bake (alternative)
pypi-build-bake: ensure-builder check-binaries
	@echo "Building PyPI packages with docker-bake..."
	@PYPI_VERSION=$(PYPI_VERSION) OUTPUT_DIR=$(OUTPUT_DIR) \
		docker buildx bake --file $(BAKE_FILE) pypi-packages

# Publish PyPI packages
pypi-publish:
	@echo "Publishing PyPI packages..."
	@if [ -z "$(PYPI_TOKEN)" ]; then \
		echo "Error: PYPI_TOKEN is required for publishing"; \
		exit 1; \
	fi
	@docker run --rm \
		-v $(OUTPUT_DIR)/pypi:/packages \
		-e TWINE_USERNAME=__token__ \
		-e TWINE_PASSWORD=$(PYPI_TOKEN) \
		python:3.12-slim \
		sh -c "pip install twine && twine upload /packages/*"

pypi-publish-test:
	@echo "Publishing PyPI packages to TestPyPI..."
	@if [ -z "$(PYPI_TOKEN)" ]; then \
		echo "Error: PYPI_TOKEN is required for publishing"; \
		exit 1; \
	fi
	@docker run --rm \
		-v $(OUTPUT_DIR)/pypi:/packages \
		-e TWINE_USERNAME=__token__ \
		-e TWINE_PASSWORD=$(PYPI_TOKEN) \
		python:3.12-slim \
		sh -c "pip install twine && twine upload --repository testpypi /packages/*"

# =============================================================================
# Combined Targets
# =============================================================================

.PHONY: all-build all-publish

all-build: npm-build pypi-build
	@echo "All packages built successfully!"
	@echo "Output directory: $(OUTPUT_DIR)"
	@ls -la $(OUTPUT_DIR)/

all-publish: all-build npm-publish pypi-publish
	@echo "All packages published!"

# =============================================================================
# Utility Targets
# =============================================================================

.PHONY: check-binaries list-binaries clean clean-output

# Check if binaries exist
check-binaries:
	@echo "Checking for binaries in $(BINARIES_DIR)..."
	@if [ ! -d "$(BINARIES_DIR)" ]; then \
		echo "Error: Binaries directory not found: $(BINARIES_DIR)"; \
		echo "Please place pre-built binaries with the following naming convention:"; \
		echo "  - injectived-darwin-arm64"; \
		echo "  - injectived-linux-arm64"; \
		echo "  - injectived-linux-x64"; \
		echo "  - libwasmvm.dylib"; \
		echo "  - libwasmvm.aarch64.so"; \
		echo "  - libwasmvm.x86_64.so"; \
		exit 1; \
	fi
	@missing=0; \
	for binary in injectived-darwin-arm64 injectived-linux-arm64 injectived-linux-x64; do \
		if [ ! -f "$(BINARIES_DIR)/$$binary" ]; then \
			echo "Warning: Binary not found: $$binary"; \
			missing=1; \
		else \
			echo "Found: $$binary"; \
		fi; \
	done; \
	lib_missing=0; \
	for lib in libwasmvm.dylib libwasmvm.aarch64.so libwasmvm.x86_64.so; do \
		if [ ! -f "$(BINARIES_DIR)/$$lib" ]; then \
			echo "Warning: WasmVM library not found: $$lib"; \
			lib_missing=1; \
		else \
			echo "Found: $$lib"; \
		fi; \
	done; \
	if [ $$missing -eq 1 ] || [ $$lib_missing -eq 1 ]; then \
		echo ""; \
		echo "Some binaries or wasmvm libraries are missing. Build them first or use the GitHub Actions workflow."; \
	fi

list-binaries:
	@echo "Available binaries:"
	@ls -la $(BINARIES_DIR)/ 2>/dev/null || echo "No binaries directory yet"

clean-output:
	@echo "Removing output directory..."
	rm -rf $(OUTPUT_DIR)

clean: clean-output
	@echo "Cleaning all build artifacts..."
	@docker buildx rm $(BUILDER_NAME) 2>/dev/null || true
