#!/usr/bin/env node

import { spawnSync } from "child_process";
import * as path from "path";
import * as fs from "fs";

/**
 * Mapping of platform and architecture to npm package names
 */
const PLATFORM_PACKAGES: Record<string, string> = {
  "darwin-arm64": "injective-cli-darwin-arm64",
  "linux-arm64": "injective-cli-linux-arm64",
  "linux-x64": "injective-cli-linux-x64",
  "win32-x64": "injective-cli-windows-x64",
  "win32-arm64": "injective-cli-windows-arm64",
};

/**
 * Get the binary name based on the platform
 */
function getBinaryName(): string {
  return process.platform === "win32" ? "injectived.exe" : "injectived";
}

function getPlatformBinaryName(): string | null {
  const key = `${process.platform}-${process.arch}`;
  if (!PLATFORM_PACKAGES[key]) {
    return null;
  }

  if (process.platform === "win32") {
    return `injectived-${key}.exe`;
  }

  return `injectived-${key}`;
}

/**
 * Get the expected npm package name for the current platform
 */
function getPlatformPackageName(): string {
  const key = `${process.platform}-${process.arch}`;
  return PLATFORM_PACKAGES[key];
}

/**
 * Find the binary path from optional dependencies
 */
function findBinaryInOptionalDeps(): string | null {
  const pkgName = getPlatformPackageName();
  if (!pkgName) {
    return null;
  }

  try {
    // Try to resolve the platform-specific package
    const pkgPath = require.resolve(`${pkgName}/package.json`);
    const pkgDir = path.dirname(pkgPath);
    const binaryPath = path.join(pkgDir, "bin", getBinaryName());
    if (fs.existsSync(binaryPath)) {
      return binaryPath;
    }

    const platformBinaryName = getPlatformBinaryName();
    if (platformBinaryName) {
      const platformBinaryPath = path.join(pkgDir, "bin", platformBinaryName);
      if (fs.existsSync(platformBinaryPath)) {
        return platformBinaryPath;
      }
    }
  } catch (e) {
    // Package not found
  }
  return null;
}

/**
 * Find the binary in the fallback location (downloaded by postinstall)
 */
function findBinaryInFallback(): string | null {
  const fallbackDir = path.join(__dirname, "..", "bin");
  const binaryPath = path.join(fallbackDir, getBinaryName());
  
  if (fs.existsSync(binaryPath)) {
    return binaryPath;
  }

  const platformBinaryName = getPlatformBinaryName();
  if (platformBinaryName) {
    const platformBinaryPath = path.join(fallbackDir, platformBinaryName);
    if (fs.existsSync(platformBinaryPath)) {
      return platformBinaryPath;
    }
  }
  return null;
}

const WASMVM_LIB_NAMES = [
  "libwasmvm.x86_64.so",
  "libwasmvm.aarch64.so",
  "libwasmvm.dylib",
];

function hasWasmvmLib(binDir: string): boolean {
  return WASMVM_LIB_NAMES.some((name) => fs.existsSync(path.join(binDir, name)));
}

function prependEnvPath(env: NodeJS.ProcessEnv, key: string, value: string): void {
  const current = env[key];
  env[key] = current ? `${value}${path.delimiter}${current}` : value;
}

function buildRuntimeEnv(binaryPath: string): NodeJS.ProcessEnv {
  const env = { ...process.env };
  const binDir = path.dirname(binaryPath);

  if (!hasWasmvmLib(binDir)) {
    return env;
  }

  if (process.platform === "linux") {
    prependEnvPath(env, "LD_LIBRARY_PATH", binDir);
  } else if (process.platform === "darwin") {
    prependEnvPath(env, "DYLD_LIBRARY_PATH", binDir);
    prependEnvPath(env, "DYLD_FALLBACK_LIBRARY_PATH", binDir);
  }

  return env;
}

/**
 * Get the binary path, either from optional deps or fallback
 */
function getBinaryPath(): string {
  // First try optional dependencies
  const optionalDepPath = findBinaryInOptionalDeps();
  if (optionalDepPath) {
    return optionalDepPath;
  }

  // Then try fallback location
  const fallbackPath = findBinaryInFallback();
  if (fallbackPath) {
    return fallbackPath;
  }

  // Error out with helpful message
  const pkgName = getPlatformPackageName();
  if (!pkgName) {
    throw new Error(
      `Unsupported platform: ${process.platform}-${process.arch}. ` +
      `Supported platforms: ${Object.keys(PLATFORM_PACKAGES).join(", ")}`
    );
  }

  throw new Error(
    `Could not find injectived binary for ${process.platform}-${process.arch}. ` +
    `Tried to find package "${pkgName}" in optionalDependencies and fallback location. ` +
    `Please ensure the platform-specific package is installed or try reinstalling injective-cli.`
  );
}

/**
 * Run the injectived binary with the provided arguments
 */
function run(): void {
  const args = process.argv.slice(2);
  const binaryPath = getBinaryPath();
  
  const result = spawnSync(binaryPath, args, { 
    stdio: "inherit",
    env: buildRuntimeEnv(binaryPath)
  });
  
  process.exit(result.status ?? 0);
}

// Run if called directly
if (require.main === module) {
  run();
}

export { getBinaryPath, findBinaryInOptionalDeps, findBinaryInFallback };
