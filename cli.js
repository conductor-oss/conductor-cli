#!/usr/bin/env node

const { spawnSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const platform = process.platform;
const binaryName = platform === 'win32' ? 'orkes.exe' : 'orkes';
const binaryPath = path.join(__dirname, 'bin', binaryName);

// Check if binary exists
if (!fs.existsSync(binaryPath)) {
  console.error('Error: Binary not found. Please reinstall the package.');
  console.error('Run: npm uninstall -g @mp-orkes/conductor-cli && npm install -g @mp-orkes/conductor-cli');
  process.exit(1);
}

// Execute the binary with all arguments
const result = spawnSync(binaryPath, process.argv.slice(2), {
  stdio: 'inherit',
  shell: false
});

process.exit(result.status);
