#!/usr/bin/env node

/**
 * Post-install script that downloads the binary as a fallback
 * when optional dependencies are not available (e.g., --no-optional flag)
 */

import * as https from "https";
import * as fs from "fs";
import * as path from "path";
import { execSync } from "child_process";

const VERSION = require("../package.json").version;
const REPO = "InjectiveFoundation/injective-core";

// Mapping of platform/arch to GitHub release asset names
const PLATFORM_ASSETS: Record<string, string> = {
  "darwin-arm64": "injectived-darwin-arm64",
  "linux-arm64": "injectived-linux-arm64",
  "linux-x64": "injectived-linux-x64",
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

function getAssetName(): string | null {
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
  } catch (e) {
    // Continue with download
  }

  // Skip if already downloaded
  if (fs.existsSync(binaryPath)) {
    console.log("injectived binary already exists in fallback location.");
    process.exit(0);
  }

  const assetName = getAssetName();
  if (!assetName) {
    console.warn(
      `Platform ${process.platform}-${process.arch} not supported for automatic download. ` +
      `Supported platforms: ${Object.keys(PLATFORM_ASSETS).join(", ")}`
    );
    process.exit(0);
  }

  // Construct download URL from GitHub releases
  // Format: https://github.com/InjectiveFoundation/injective-core/releases/download/v{version}/{assetName}
  const tag = `v${VERSION}`;
  const downloadUrl = `https://github.com/${REPO}/releases/download/${tag}/${assetName}`;

  console.log(`Downloading injectived binary for ${process.platform}-${process.arch}...`);
  console.log(`URL: ${downloadUrl}`);

  // Create bin directory if it doesn't exist
  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  try {
    await downloadFile({ url: downloadUrl, dest: binaryPath });
    
    // Make binary executable on Unix
    if (process.platform !== "win32") {
      fs.chmodSync(binaryPath, 0o755);
    }

    console.log(`Successfully installed injectived at ${binaryPath}`);
  } catch (error) {
    console.warn(`Failed to download binary: ${error}`);
    console.warn("You may need to manually install the binary or use the platform-specific npm package.");
    // Don't fail the install, just warn
    process.exit(0);
  }
}

main().catch((err) => {
  console.warn(`Post-install script warning: ${err}`);
  process.exit(0); // Don't fail the install
});
