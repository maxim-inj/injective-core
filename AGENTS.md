# AI Guidelines.md

This file provides guidance to AI agents like Claude Code when working with code in this repository.

## About Injective Core

Injective Core is the backbone of the Injective Protocol, a layer-1 blockchain built on the Cosmos SDK optimized for DeFi applications. It includes:

- **injectived**: The main Injective blockchain node daemon
- **peggo**: The Peggy orchestrator for Ethereum-Injective bridge operations

The chain supports advanced financial primitives including spot markets, perpetual futures, expiry futures, binary options, and a sophisticated exchange module.

## Common Commands

### Build and Install

```bash
# Install both injectived and peggo (recommended for local development)
make install

# Install only injectived
make install-injectived

# Install only peggo
make install-peggo

# Build Docker image
make image

# Install for CI (with coverage support)
DO_COVERAGE=true make install-ci
```

### Testing

```bash
# Run all tests (unit + interchain tests)
make test

# Run unit tests only (using Ginkgo)
make test-unit

# Run exchange module tests
make test-exchange

# Run fuzz tests
make test-fuzz

# Run RPC tests
make test-rpc

# Generate coverage report
make cover
```

### Interchain Tests (E2E)

```bash
# Run all interchain tests
make ictest-all

# Run specific interchain tests
make ictest-basic          # Basic chain start test
make ictest-upgrade        # Upgrade handler test
make ictest-peggo          # Peggy bridge test
make ictest-evm            # EVM/RPC tests
make ictest-lanes          # Mempool lanes test
make ictest-hyperlane      # Hyperlane cross-chain test
make ictest-chainstream    # Chain stream test
make ictest-fixed-gas      # Fixed gas test
```

### Linting and Formatting

```bash
# Run linter (checks against master branch)
make lint

# Run linter against last commit only
make lint-last-commit

# Note: Uses golangci-lint v2.1.6 with 15 minute timeout
# Configuration is in .golangci.yml
```

### Protobuf Generation

```bash
# Generate all protobuf files
make proto

# Generate protobuf Go bindings
make proto-gen

# Generate Swagger documentation
make proto-swagger-gen

# Format proto files
make proto-format

# Lint proto files
make proto-lint

# Check for breaking changes
make proto-check-breaking
```

### Mock Generation

```bash
# Generate mocks for testing
make mock
```

### Other Useful Commands

```bash
# Initialize git hooks
make init

# Generate precompile bindings
make precompiles-bindings

# Generate Peggy contract wrappers
make peggo-wrappers

# Generate error documentation
make gen-error-docs

# Launch gRPC UI for debugging
make grpc-ui
```

## Architecture Overview

Injective Core is a Cosmos SDK-based blockchain with specialized modules for decentralized finance. The architecture follows the standard Cosmos SDK module pattern with custom extensions.

### Core Design Principles

1. **Cosmos SDK Modules**: Each feature is implemented as a self-contained module with keeper, types, and handlers
2. **Event-Driven**: Uses Cosmos SDK events for tracking state changes
3. **Cross-Module Communication**: Modules interact through keeper interfaces
4. **Deterministic Execution**: All state changes must be deterministic for consensus
5. **Gas Efficiency**: Optimized for high-frequency trading operations

### Key Components

#### Blockchain Node (injectived)

The main chain daemon located in `cmd/injectived/`:

- `root.go`: CLI root command setup
- `start.go`: Node startup logic
- `config/`: Chain configuration

#### Exchange Module

The core trading engine in `injective-chain/modules/exchange/`:

- `keeper/`: State management and business logic
  - `base/`: Base keeper functionality
  - `spot/`: Spot market operations
  - `derivative/`: Derivative (perpetual/expiry) operations
  - `binaryoptions/`: Binary options trading
  - `subaccount/`: Subaccount management
  - `feediscounts/`: Fee discount calculations
  - `rewards/`: Trading rewards
- `types/`: Protobuf types and validation
- `testexchange/`: Test utilities and fixtures

#### Other Key Modules

- `auction/`: Token burn auctions
- `peggy/`: Ethereum bridge (Peggy)
- `oracle/`: Price feed oracles
- `insurance/`: Insurance fund management
- `tokenfactory/`: Custom token creation
- `wasmx/`: CosmWasm extensions
- `evm/`: EVM compatibility layer
- `erc20/`: ERC20 token support
- `permissions/`: Permission management
- `txfees/`: Transaction fee handling
- `ocr/`: Off-chain reporting
- `downtime-detector/`: Chain downtime detection

#### Peggo Orchestrator

The Ethereum-Injective bridge orchestrator in `peggo/`:

- `orchestrator/`: Main orchestrator logic
  - `cosmos/`: Injective chain interaction
  - `ethereum/`: Ethereum chain interaction
  - `loops/`: Background processing loops
- `monitor/`: Bridge monitoring
- `solidity/`: Contract ABIs and bindings

#### Interchain Tests

End-to-end tests using strangelove-ventures/interchaintest in `interchaintest/`:

- `setup.go`: Test infrastructure setup
- `helpers/`: Test helper functions
- Individual test files for each feature

### Application Architecture

```
injective-chain/
├── app/              # Main application wiring
│   ├── app.go        # Module registration, keeper setup
│   ├── upgrades/     # Chain upgrade handlers
│   └── ante/         # Transaction ante handlers
├── modules/          # Custom Injective modules
├── types/            # Shared types
├── stream/           # Chain streaming (gRPC)
├── websocket/        # WebSocket server
└── lanes/            # Block-SDK mempool lanes
```

## Code Organization

### Module Structure

Each module follows the Cosmos SDK convention:

```
modules/<module>/
├── keeper/           # State management
│   ├── keeper.go     # Main keeper
│   ├── msg_server.go # Transaction handlers
│   └── grpc_query.go # Query handlers
├── types/            # Types and validation
│   ├── msgs.go       # Message definitions
│   ├── keys.go       # Store keys
│   └── errors.go     # Module-specific errors
├── client/           # CLI commands
├── module.go         # Module interface
└── abci.go           # ABCI hooks (if any)
```

### Proto Files

Protocol buffer definitions in `proto/injective/`:

- Each module has its own subdirectory
- Follow Cosmos SDK proto conventions
- Generate Go code with `make proto-gen`

## Code Style and Best Practices

When contributing to Injective Core, follow these Go and Cosmos SDK best practices:

### General Principles

1. **Follow Go Idioms**: Use standard Go patterns and conventions
2. **Error Handling**: Return errors with context using `sdkerrors.Wrap`
3. **Determinism**: All keeper methods must be deterministic
4. **Gas Metering**: Be aware of gas costs for storage operations
5. **Event Emission**: Emit appropriate events for all state changes
6. **Simplicity First**: Choose the simplest solution that solves the problem

### Specific Guidelines

1. **Naming**:

   - Use clear, descriptive names
   - Follow Go naming conventions (camelCase for functions, PascalCase for exports)
   - Module-specific types should be prefixed appropriately

2. **Error Handling**:

   - Use `sdkerrors.Wrap` and `sdkerrors.Wrapf` for errors
   - Create module-specific error codes in `types/errors.go`
   - Never panic in production code

3. **State Management**:

   - Use keepers for all state access
   - Prefix store keys appropriately
   - Use collections where appropriate

4. **Testing**:

   - Write unit tests with Ginkgo/Gomega
   - Use table-driven tests for comprehensive coverage
   - Test edge cases and error conditions
   - Use `testexchange` fixtures for exchange module tests

5. **Comments**:
   - Document all exported functions
   - Use `// TODO:` and `// FIXME:` for future work
   - Explain "why", not "what"

### Cosmos SDK Patterns

1. **Message Validation**: Implement `ValidateBasic()` for all messages
2. **Ante Handlers**: Use ante handlers for pre-execution checks
3. **Hooks**: Use keeper hooks for cross-module communication
4. **Genesis**: Implement proper genesis import/export
5. **Upgrades**: Use upgrade handlers for state migrations

## Testing

### Unit Testing with Ginkgo

The project uses Ginkgo/Gomega for BDD-style testing:

```bash
# Run all unit tests
make test-unit

# Run tests for a specific package
cd injective-chain/modules/exchange
go test -v ./...
```

### Exchange Module Testing

The exchange module has extensive test utilities in `testexchange/`:

- Test fixtures and helpers
- Fuzz testing support
- Market simulation utilities

### Interchain Testing

E2E tests use the `interchaintest` framework:

- Tests run against real Docker containers
- Support for multi-chain scenarios
- Coverage collection enabled

### Coverage

```bash
# Generate coverage profile
make test-unit

# View coverage report
make cover
```

## Linting Configuration

The project uses golangci-lint v2 with these enabled linters:

- `errorlint`: Error handling best practices
- `gocritic`: Go code analysis
- `misspell`: Spelling corrections
- `prealloc`: Slice preallocation
- `revive`: Go code style
- `unconvert`: Unnecessary type conversions
- `unparam`: Unused function parameters

Configuration is in [.golangci.yml](.golangci.yml).

## Contribution Workflow

### Step 1: Set Up Development Environment

```bash
# Clone the repository
git clone git@github.com:InjectiveLabs/injective-core.git
cd injective-core

# Initialize git hooks
make init

# Install dependencies and build
make install
```

### Step 2: Development Process

1. **Branch**: Create a feature branch from `master` or `release/*`
2. **Implement**: Write code following project style guidelines
3. **Test**: Add tests covering your changes
4. **Lint**: Run `make lint` to check for issues
5. **Document**: Update CHANGELOG.md as needed

### Step 3: Pre-Commit Checks

Before committing, ensure:

```bash
# Run linter
make lint

# Run tests
make test-unit

# For protobuf changes
make proto-format
make proto-lint
```

### Step 4: Submit Changes

1. **Commit**: Use clear commit messages
2. **Push**: Push to your feature branch
3. **PR**: Open a pull request
4. **CI**: Ensure all CI checks pass

### Commit Message Guidelines

Follow standard conventions:

- Use present tense ("Add feature", not "Added feature")
- First line is a summary (50 chars or less)
- Include module prefix when applicable (e.g., `exchange:`, `peggo:`)
- Reference issues when applicable

### CHANGELOG Guidelines

Follow the format in CHANGELOG.md:

- Add entries under "Unreleased" section
- Use appropriate stanza (Features, Bug Fixes, etc.)
- Include PR number and brief description
- Format: `(<tag>) [#<pr>](url) message`

## Development Tips

- Interchain tests provide good examples of end-to-end flows
- The exchange module `testexchange/` has utilities for market testing

### Debugging

- Enable debug logging in config
- Use `grpcui` for API exploration
- Chain stream provides real-time event monitoring
- Check Prometheus metrics at the configured endpoint

### Common Pitfalls

1. **Determinism**: Avoid maps in iteration (non-deterministic order)
2. **Gas**: Large state operations can exceed gas limits
3. **Events**: Always emit events for state changes
4. **Upgrades**: Test upgrade handlers thoroughly
5. **Proto**: Regenerate protos after `.proto` file changes

## Universal Code Quality Principles

### Production Safety Requirements

- **Always handle errors**: Use `sdkerrors.Wrap` with context
- **Avoid panics**: Never panic in production code paths
- **Validate inputs**: Implement `ValidateBasic()` for all messages
- **Be deterministic**: Avoid non-deterministic operations in state transitions
- **Memory safety**: Be careful with large allocations

### API and Dependency Management

- **Check go.mod for versions**: Verify dependency versions before using APIs
- **Follow Cosmos SDK patterns**: Use established patterns from the SDK
- **Search existing code**: Look for existing patterns before implementing
- **Don't assume**: Verify APIs exist before suggesting them

### Incremental Improvement Strategy

- **Fix issues in touched code**: Improve code you're modifying
- **Use best practices for new code**: Don't add technical debt
- **Create issues for unrelated debt**: Don't fix unrelated code in current PR
