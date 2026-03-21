#!/usr/bin/env node
// Downloads the ghx Go binary from GitHub releases on npm install
const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const https = require("https");

const REPO = "gkoreli/ghx";
const BIN = path.join(__dirname, "ghx");

const PLATFORM_MAP = { darwin: "darwin", linux: "linux", win32: "windows" };
const ARCH_MAP = { x64: "amd64", arm64: "arm64" };

const os = PLATFORM_MAP[process.platform];
const arch = ARCH_MAP[process.arch];
if (!os || !arch) {
  console.error(`Unsupported platform: ${process.platform}/${process.arch}`);
  process.exit(1);
}

const ext = os === "windows" ? "zip" : "tar.gz";
const url = `https://github.com/${REPO}/releases/latest/download/ghx_${os}_${arch}.${ext}`;

function download(url, dest) {
  return new Promise((resolve, reject) => {
    https.get(url, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        return download(res.headers.location, dest).then(resolve, reject);
      }
      if (res.statusCode !== 200) return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
      const file = fs.createWriteStream(dest);
      res.pipe(file);
      file.on("finish", () => file.close(resolve));
    }).on("error", reject);
  });
}

async function main() {
  const tmp = path.join(__dirname, `ghx-download.${ext}`);
  try {
    console.log(`Downloading ghx (${os}/${arch})...`);
    await download(url, tmp);
    if (ext === "tar.gz") {
      execSync(`tar xzf "${tmp}" -C "${__dirname}" ghx`, { stdio: "pipe" });
    } else {
      execSync(`unzip -o "${tmp}" ghx.exe -d "${__dirname}"`, { stdio: "pipe" });
    }
    fs.chmodSync(BIN, 0o755);
    console.log("ghx installed successfully");
  } finally {
    try { fs.unlinkSync(tmp); } catch {}
  }
}

main().catch((e) => {
  console.error(`Failed to install ghx: ${e.message}`);
  console.error("Install manually: https://github.com/gkoreli/ghx#install");
  process.exit(1);
});
