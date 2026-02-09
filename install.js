#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');

const REPO = 'conductor-oss/conductor-cli';
const BINARY_NAME = 'conductor';

// Detect platform and architecture
function getPlatform() {
  const platform = process.platform;
  const arch = process.arch;

  const platformMap = {
    darwin: 'darwin',
    linux: 'linux',
    win32: 'windows'
  };

  const archMap = {
    x64: 'amd64',
    arm64: 'arm64'
  };

  if (!platformMap[platform]) {
    console.error(`Unsupported platform: ${platform}`);
    process.exit(1);
  }

  if (!archMap[arch]) {
    console.error(`Unsupported architecture: ${arch}`);
    process.exit(1);
  }

  return {
    os: platformMap[platform],
    arch: archMap[arch],
    isWindows: platform === 'win32'
  };
}

// Download binary following redirects
function downloadBinary(url, dest) {
  return new Promise((resolve, reject) => {
    const makeRequest = (requestUrl) => {
      const mod = requestUrl.startsWith('https') ? https : require('http');
      mod.get(requestUrl, { headers: { 'User-Agent': 'conductor-cli-npm-installer' } }, (response) => {
        if (response.statusCode === 302 || response.statusCode === 301) {
          makeRequest(response.headers.location);
          return;
        }

        if (response.statusCode !== 200) {
          reject(new Error(`Failed to download: ${response.statusCode}`));
          return;
        }

        const file = fs.createWriteStream(dest);
        response.pipe(file);

        file.on('finish', () => {
          file.close();
          resolve();
        });

        file.on('error', (err) => {
          fs.unlink(dest, () => {});
          reject(err);
        });
      }).on('error', (err) => {
        fs.unlink(dest, () => {});
        reject(err);
      });
    };

    makeRequest(url);
  });
}

async function install() {
  try {
    console.log('Installing Conductor CLI...');

    const { os, arch, isWindows } = getPlatform();
    console.log(`Platform: ${os} ${arch}`);

    // Use GitHub's latest release redirect URL to avoid API rate limits
    const binaryName = isWindows ? `${BINARY_NAME}.exe` : BINARY_NAME;
    const downloadName = isWindows ? `${BINARY_NAME}_${os}_${arch}.exe` : `${BINARY_NAME}_${os}_${arch}`;
    const downloadUrl = `https://github.com/${REPO}/releases/latest/download/${downloadName}`;

    console.log(`Downloading from: ${downloadUrl}`);

    // Create bin directory
    const binDir = path.join(__dirname, 'bin');
    if (!fs.existsSync(binDir)) {
      fs.mkdirSync(binDir, { recursive: true });
    }

    // Download binary
    const binaryPath = path.join(binDir, binaryName);
    await downloadBinary(downloadUrl, binaryPath);

    // Make executable (Unix-like systems)
    if (!isWindows) {
      fs.chmodSync(binaryPath, 0o755);
    }

    console.log('Installation successful!');
    console.log(`Binary installed at: ${binaryPath}`);
    console.log(`\nRun 'conductor --version' to verify installation.`);
  } catch (error) {
    console.error('Installation failed:', error.message);
    process.exit(1);
  }
}

install();
