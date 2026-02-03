#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

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

// Get latest release version
function getLatestVersion() {
  return new Promise((resolve, reject) => {
    const options = {
      hostname: 'api.github.com',
      path: `/repos/${REPO}/releases/latest`,
      method: 'GET',
      headers: {
        'User-Agent': 'conductor-cli-npm-installer'
      }
    };

    https.get(options, (res) => {
      let data = '';

      res.on('data', (chunk) => {
        data += chunk;
      });

      res.on('end', () => {
        try {
          const json = JSON.parse(data);
          resolve(json.tag_name);
        } catch (e) {
          reject(new Error('Failed to parse release data'));
        }
      });
    }).on('error', (err) => {
      reject(err);
    });
  });
}

// Download binary
function downloadBinary(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);

    https.get(url, (response) => {
      if (response.statusCode === 302 || response.statusCode === 301) {
        // Follow redirect
        return downloadBinary(response.headers.location, dest).then(resolve).catch(reject);
      }

      if (response.statusCode !== 200) {
        reject(new Error(`Failed to download: ${response.statusCode}`));
        return;
      }

      response.pipe(file);

      file.on('finish', () => {
        file.close();
        resolve();
      });
    }).on('error', (err) => {
      fs.unlink(dest, () => {});
      reject(err);
    });
  });
}

async function install() {
  try {
    console.log('Installing Conductor CLI...');

    const { os, arch, isWindows } = getPlatform();
    console.log(`Platform: ${os} ${arch}`);

    // Get latest version
    console.log('Fetching latest version...');
    const version = await getLatestVersion();
    console.log(`Latest version: ${version}`);

    // Construct download URL
    const binaryName = isWindows ? `${BINARY_NAME}.exe` : BINARY_NAME;
    const downloadName = isWindows ? `${BINARY_NAME}_${os}_${arch}.exe` : `${BINARY_NAME}_${os}_${arch}`;
    const downloadUrl = `https://github.com/${REPO}/releases/download/${version}/${downloadName}`;

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
