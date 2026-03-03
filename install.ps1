# Conductor CLI Installer for Windows
# Usage: irm https://raw.githubusercontent.com/conductor-oss/conductor-cli/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

Write-Host "Installing Conductor CLI..." -ForegroundColor Cyan

# Get latest version from GitHub
try {
    $release = Invoke-RestMethod "https://api.github.com/repos/conductor-oss/conductor-cli/releases/latest"
    $version = $release.tag_name
} catch {
    Write-Host "Error: Failed to fetch latest version from GitHub" -ForegroundColor Red
    exit 1
}

Write-Host "Latest version: $version"

# Detect architecture
$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "arm64" }
Write-Host "Architecture: windows_$arch"

# Download URL
$url = "https://github.com/conductor-oss/conductor-cli/releases/download/$version/conductor_windows_$arch.exe"

# Install directory (user-local, no admin required)
$installDir = "$env:LOCALAPPDATA\conductor"

# Create directory if it doesn't exist
if (!(Test-Path $installDir)) {
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
}

$exePath = "$installDir\conductor.exe"

# Download the binary
Write-Host "Downloading from $url..."
try {
    Invoke-WebRequest -Uri $url -OutFile $exePath -UseBasicParsing
} catch {
    Write-Host "Error: Failed to download conductor CLI" -ForegroundColor Red
    exit 1
}

# Add to user PATH if not already there
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    Write-Host "Adding $installDir to user PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    $env:Path = "$env:Path;$installDir"
}

# Verify installation
Write-Host ""
Write-Host "Conductor CLI $version installed successfully!" -ForegroundColor Green
Write-Host "Location: $exePath"
Write-Host ""
Write-Host "Restart your terminal, then run:" -ForegroundColor Yellow
Write-Host "  conductor --version"
