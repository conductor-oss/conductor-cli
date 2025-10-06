#!/bin/sh
set -e

REPO="conductor-oss/conductor-cli"
BINARY_NAME="orkes"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

detect_os() {
    OS="$(uname -s)"
    case "$OS" in
        Linux*)     OS='linux';;
        Darwin*)    OS='darwin';;
        CYGWIN*)    OS='windows';;
        MINGW*)     OS='windows';;
        *)
            echo "${RED}Unsupported operating system: $OS${NC}"
            exit 1
            ;;
    esac
}

# Detect architecture
detect_arch() {
    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64)     ARCH='amd64';;
        amd64)      ARCH='amd64';;
        arm64)      ARCH='arm64';;
        aarch64)    ARCH='arm64';;
        *)
            echo "${RED}Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac
}

get_latest_version() {
    VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "${RED}Failed to get latest version${NC}"
        exit 1
    fi
    echo "${GREEN}Latest version: $VERSION${NC}"
}

install_binary() {
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/${BINARY_NAME}_${OS}_${ARCH}"

    if [ "$OS" = "windows" ]; then
        DOWNLOAD_URL="${DOWNLOAD_URL}.exe"
        BINARY_NAME="${BINARY_NAME}.exe"
    fi

    echo "${YELLOW}Downloading from: $DOWNLOAD_URL${NC}"

    TMP_DIR=$(mktemp -d)
    TMP_FILE="$TMP_DIR/$BINARY_NAME"

    # Download binary
    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"; then
        echo "${RED}Failed to download binary${NC}"
        rm -rf "$TMP_DIR"
        exit 1
    fi

    # Make executable
    chmod +x "$TMP_FILE"

    # Check if install directory is writable
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
        echo "${GREEN}Installed $BINARY_NAME to $INSTALL_DIR${NC}"
    else
        echo "${YELLOW}$INSTALL_DIR is not writable. Attempting to use sudo...${NC}"
        sudo mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
        echo "${GREEN}Installed $BINARY_NAME to $INSTALL_DIR (with sudo)${NC}"
    fi

    rm -rf "$TMP_DIR"
}

verify_installation() {
    if command -v $BINARY_NAME >/dev/null 2>&1; then
        VERSION_OUTPUT=$($BINARY_NAME --version 2>&1 || true)
        echo "${GREEN}✓ Installation successful!${NC}"
        echo "${GREEN}  Version: $VERSION_OUTPUT${NC}"
        echo ""
        echo "Run '${BINARY_NAME} --help' to get started."
    else
        echo "${YELLOW}⚠ Binary installed but not found in PATH${NC}"
        echo "You may need to add $INSTALL_DIR to your PATH:"
        echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    fi
}

main() {
    echo "${GREEN}Installing Orkes CLI...${NC}"
    echo ""

    detect_os
    detect_arch

    echo "Detected OS: $OS"
    echo "Detected Architecture: $ARCH"
    echo ""

    get_latest_version
    install_binary
    verify_installation
}

main
