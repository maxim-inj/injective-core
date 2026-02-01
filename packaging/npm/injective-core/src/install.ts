#!/usr/bin/env node

/**
 * Post-install script that downloads the binary as a fallback
 * when optional dependencies are not available (e.g., --no-optional flag)
 */

import * as https from "https";
import * as fs from "fs";
import * as path from "path";
import * as os from "os";

const VERSION = require("../package.json").version;
const REPO = "InjectiveFoundation/injective-core";
const AdmZip = require("adm-zip");

// Mapping of platform/arch to GitHub release asset names
const PLATFORM_ASSETS: Record<string, { zip: string; wasmvm: string }> = {
  "darwin-arm64": { zip: "darwin-arm64.zip", wasmvm: "libwasmvm.dylib" },
  "linux-arm64": { zip: "linux-arm64.zip", wasmvm: "libwasmvm.aarch64.so" },
  "linux-x64": { zip: "linux-amd64.zip", wasmvm: "libwasmvm.x86_64.so" },
};

interface DownloadOptions {
  url: string;
  dest: string;
}

function downloadFile(options: DownloadOptions): Promise<void> {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(options.dest);
    
    https
      .get(options.url, (response) => {
        if (response.statusCode === 301 || response.statusCode === 302) {
          const redirectUrl = response.headers.location;
          if (redirectUrl) {
            downloadFile({ ...options, url: redirectUrl })
              .then(resolve)
              .catch(reject);
            return;
          }
        }

        if (response.statusCode !== 200) {
          reject(new Error(`Download failed with status ${response.statusCode}`));
          return;
        }

        response.pipe(file);
        file.on("finish", () => {
          file.close();
          resolve();
        });
      })
      .on("error", (err) => {
        fs.unlink(options.dest, () => {});
        reject(err);
      });
  });
}

function getAssetInfo(): { zip: string; wasmvm: string } | null {
  const key = `${process.platform}-${process.arch}`;
  return PLATFORM_ASSETS[key] || null;
}

async function main(): Promise<void> {
  // Check if binary already exists (optional dependency was installed)
  const binDir = path.join(__dirname, "..", "bin");
  const binaryName = process.platform === "win32" ? "injectived.exe" : "injectived";
  const binaryPath = path.join(binDir, binaryName);

  // Check if we can find the binary from optional dependencies
  try {
    const indexModule = require("./index");
    if (indexModule.findBinaryInOptionalDeps()) {
      console.log("injectived binary found in optional dependencies.");
      process.exit(0);
    }
    if (typeof indexModule.ensureBinaryInOptionalDeps === "function") {
      const ensured = await indexModule.ensureBinaryInOptionalDeps();
      if (ensured) {
        console.log("injectived binary unpacked from optional dependencies.");
        process.exit(0);
      }
    }
  } catch (e) {
    // Continue with download
  }

  const assetInfo = getAssetInfo();
  if (!assetInfo) {
    console.warn(
      `Platform ${process.platform}-${process.arch} not supported for automatic download. ` +
      `Supported platforms: ${Object.keys(PLATFORM_ASSETS).join(", ")}`
    );
    process.exit(0);
  }

  const wasmvmPath = path.join(binDir, assetInfo.wasmvm);

  // Skip if already downloaded
  if (fs.existsSync(binaryPath) && fs.existsSync(wasmvmPath)) {
    console.log("injectived binary and wasmvm library already exist in fallback location.");
    process.exit(0);
  }

  // Construct download URL from GitHub releases
  // Format: https://github.com/InjectiveFoundation/injective-core/releases/download/v{version}/{zip}
  const tag = `v${VERSION}`;
  const downloadUrl = `https://github.com/${REPO}/releases/download/${tag}/${assetInfo.zip}`;

  console.log(`Downloading injectived binary for ${process.platform}-${process.arch}...`);
  console.log(`URL: ${downloadUrl}`);

  // Create bin directory if it doesn't exist
  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  const zipPath = path.join(binDir, assetInfo.zip);
  const extractDir = fs.mkdtempSync(path.join(os.tmpdir(), "injective-core-"));

  try {
    await downloadFile({ url: downloadUrl, dest: zipPath });

    const zip = new AdmZip(zipPath);
    zip.extractAllTo(extractDir, true);

    const extractedBinary = path.join(extractDir, binaryName);
    const extractedWasmvm = path.join(extractDir, assetInfo.wasmvm);

    if (!fs.existsSync(extractedBinary) || !fs.existsSync(extractedWasmvm)) {
      throw new Error(`Expected files not found in ${assetInfo.zip}`);
    }

    fs.copyFileSync(extractedBinary, binaryPath);
    fs.copyFileSync(extractedWasmvm, wasmvmPath);

    if (process.platform !== "win32") {
      fs.chmodSync(binaryPath, 0o755);
      const platformBinaryName = `injectived-${process.platform}-${process.arch}`;
      const platformBinaryPath = path.join(binDir, platformBinaryName);
      try {
        if (!fs.existsSync(platformBinaryPath)) {
          fs.symlinkSync(binaryName, platformBinaryPath);
        }
      } catch {
        // ignore symlink errors
      }
    }

    console.log(`Successfully installed injectived at ${binaryPath}`);
  } catch (error) {
    console.warn(`Failed to download binary: ${error}`);
    console.warn("You may need to manually install the binary or use the platform-specific npm package.");
    // Don't fail the install, just warn
    process.exit(0);
  } finally {
    try {
      fs.rmSync(extractDir, { recursive: true, force: true });
      fs.rmSync(zipPath, { force: true });
    } catch {
      // ignore cleanup errors
    }
  }
}

main().catch((err) => {
  console.warn(`Post-install script warning: ${err}`);
  process.exit(0); // Don't fail the install
});
