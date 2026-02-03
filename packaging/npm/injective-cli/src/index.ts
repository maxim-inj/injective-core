#!/usr/bin/env node

import { spawnSync } from "child_process";
import * as path from "path";
import * as fs from "fs";
import * as os from "os";

const tar = require("tar");
const { ZstdCodec } = require("zstd-codec");

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

const PAYLOAD_ARCHIVE = "injectived.tar.zst";

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
function getPlatformPackageName(): string | null {
  const key = `${process.platform}-${process.arch}`;
  return PLATFORM_PACKAGES[key] || null;
}

function findBinaryInDir(binDir: string): string | null {
  const binaryPath = path.join(binDir, getBinaryName());
  if (fs.existsSync(binaryPath)) {
    return binaryPath;
  }

  const platformBinaryName = getPlatformBinaryName();
  if (platformBinaryName) {
    const platformBinaryPath = path.join(binDir, platformBinaryName);
    if (fs.existsSync(platformBinaryPath)) {
      return platformBinaryPath;
    }
  }

  return null;
}

async function decompressPayload(archivePath: string): Promise<Buffer> {
  const compressed = fs.readFileSync(archivePath);

  return await new Promise((resolve, reject) => {
    ZstdCodec.run((zstd: { Simple: new () => { decompress: (input: Uint8Array) => Uint8Array } }) => {
      try {
        const simple = new zstd.Simple();
        const decompressed = simple.decompress(compressed);
        resolve(Buffer.from(decompressed));
      } catch (err) {
        reject(err);
      }
    });
  });
}

async function extractPayload(binDir: string): Promise<void> {
  const archivePath = path.join(binDir, PAYLOAD_ARCHIVE);

  if (!fs.existsSync(archivePath)) {
    return;
  }

  const tarBuffer = await decompressPayload(archivePath);
  const tmpTarPath = path.join(os.tmpdir(), `injective-cli-${Date.now()}-${Math.random()}.tar`);
  fs.writeFileSync(tmpTarPath, tarBuffer);

  try {
    await tar.extract({ file: tmpTarPath, cwd: binDir });
  } finally {
    fs.rmSync(tmpTarPath, { force: true });
  }

  const platformBinaryName = getPlatformBinaryName();
  if (platformBinaryName) {
    const platformBinaryPath = path.join(binDir, platformBinaryName);
    const binaryPath = path.join(binDir, getBinaryName());

    if (fs.existsSync(platformBinaryPath) && process.platform !== "win32") {
      fs.chmodSync(platformBinaryPath, 0o755);
    }

    if (!fs.existsSync(binaryPath) && fs.existsSync(platformBinaryPath)) {
      try {
        fs.symlinkSync(platformBinaryName, binaryPath);
      } catch {
        fs.copyFileSync(platformBinaryPath, binaryPath);
      }
    }
  }
}

async function ensureBinaryInDir(binDir: string): Promise<string | null> {
  const existing = findBinaryInDir(binDir);
  if (existing) {
    return existing;
  }

  await extractPayload(binDir);
  return findBinaryInDir(binDir);
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
    return findBinaryInDir(path.join(pkgDir, "bin"));
  } catch (e) {
    // Package not found
  }
  return null;
}

async function ensureBinaryInOptionalDeps(): Promise<string | null> {
  const pkgName = getPlatformPackageName();
  if (!pkgName) {
    return null;
  }

  try {
    const pkgPath = require.resolve(`${pkgName}/package.json`);
    const pkgDir = path.dirname(pkgPath);
    return await ensureBinaryInDir(path.join(pkgDir, "bin"));
  } catch {
    return null;
  }
}

/**
 * Find the binary in the fallback location (downloaded by postinstall)
 */
function findBinaryInFallback(): string | null {
  const fallbackDir = path.join(__dirname, "..", "bin");
  return findBinaryInDir(fallbackDir);
}

async function ensureBinaryInFallback(): Promise<string | null> {
  const fallbackDir = path.join(__dirname, "..", "bin");
  return await ensureBinaryInDir(fallbackDir);
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

async function ensureBinaryAvailable(): Promise<void> {
  const optionalDepPath = await ensureBinaryInOptionalDeps();
  if (optionalDepPath) {
    return;
  }

  const fallbackPath = await ensureBinaryInFallback();
  if (fallbackPath) {
    return;
  }
}

/**
 * Run the injectived binary with the provided arguments
 */
async function run(): Promise<void> {
  const args = process.argv.slice(2);
  await ensureBinaryAvailable();
  const binaryPath = getBinaryPath();
  
  const result = spawnSync(binaryPath, args, { 
    stdio: "inherit",
    env: buildRuntimeEnv(binaryPath)
  });
  
  process.exit(result.status ?? 0);
}

// Run if called directly
if (require.main === module) {
  run().catch((err) => {
    console.error(err instanceof Error ? err.message : err);
    process.exit(1);
  });
}

export {
  getBinaryPath,
  findBinaryInOptionalDeps,
  findBinaryInFallback,
  ensureBinaryInOptionalDeps,
  ensureBinaryInFallback,
  ensureBinaryAvailable,
};
