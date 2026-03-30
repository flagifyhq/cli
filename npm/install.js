const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const https = require("https");
const http = require("http");
const { createGunzip } = require("zlib");
const tar = require("tar");

const REPO = "flagifyhq/cli";
const BIN_DIR = path.join(__dirname, "bin");
const BIN_PATH = path.join(BIN_DIR, process.platform === "win32" ? "flagify.exe" : "flagify");

const PLATFORM_MAP = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

const ARCH_MAP = {
  x64: "amd64",
  arm64: "arm64",
};

function getDownloadURL(version) {
  const platform = PLATFORM_MAP[process.platform];
  const arch = ARCH_MAP[process.arch];

  if (!platform || !arch) {
    throw new Error(`Unsupported platform: ${process.platform} ${process.arch}`);
  }

  const name = `flagify_${platform}_${arch}`;
  const ext = process.platform === "win32" ? "zip" : "tar.gz";

  return `https://github.com/${REPO}/releases/download/v${version}/${name}.${ext}`;
}

function getVersion() {
  const pkg = require("./package.json");
  return pkg.version;
}

function fetch(url) {
  return new Promise((resolve, reject) => {
    const client = url.startsWith("https") ? https : http;
    client
      .get(url, { headers: { "User-Agent": "flagify-cli-npm" } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return fetch(res.headers.location).then(resolve).catch(reject);
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`Download failed: HTTP ${res.statusCode} from ${url}`));
        }
        resolve(res);
      })
      .on("error", reject);
  });
}

async function extractTarGz(stream, destDir) {
  return new Promise((resolve, reject) => {
    stream
      .pipe(createGunzip())
      .pipe(tar.extract({ cwd: destDir, strip: 0 }))
      .on("finish", resolve)
      .on("error", reject);
  });
}

async function extractZip(stream, destDir) {
  const AdmZip = require("adm-zip");
  const chunks = [];
  for await (const chunk of stream) chunks.push(chunk);
  const buffer = Buffer.concat(chunks);
  const zip = new AdmZip(buffer);
  zip.extractAllTo(destDir, true);
}

async function install() {
  const version = getVersion();
  const url = getDownloadURL(version);

  console.log(`Downloading flagify v${version}...`);

  if (!fs.existsSync(BIN_DIR)) {
    fs.mkdirSync(BIN_DIR, { recursive: true });
  }

  const stream = await fetch(url);

  if (process.platform === "win32") {
    await extractZip(stream, BIN_DIR);
  } else {
    await extractTarGz(stream, BIN_DIR);
  }

  fs.chmodSync(BIN_PATH, 0o755);
  console.log(`Installed flagify v${version} to ${BIN_PATH}`);
}

install().catch((err) => {
  console.error(`Failed to install flagify: ${err.message}`);
  process.exit(1);
});
