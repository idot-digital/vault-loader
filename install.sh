#!/bin/sh

# Exit on error
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to check if a command exists
check_command() {
    if ! command -v "$1" > /dev/null 2>&1; then
        printf "${RED}Error: %s is not installed${NC}\n" "$1"
        printf "${YELLOW}Please install %s and try again${NC}\n" "$1"
        case "$1" in
            "curl")
                printf "On Ubuntu/Debian: sudo apt-get install curl\n"
                printf "On Fedora: sudo dnf install curl\n"
                printf "On Alpine: apk add curl\n"
                ;;
            "tar")
                printf "On Ubuntu/Debian: sudo apt-get install tar\n"
                printf "On Fedora: sudo dnf install tar\n"
                printf "On Alpine: apk add tar\n"
                ;;
            "sudo")
                printf "On Ubuntu/Debian: sudo apt-get install sudo\n"
                printf "On Fedora: sudo dnf install sudo\n"
                printf "On Alpine: apk add sudo\n"
                ;;
        esac
        exit 1
    fi
}

# Check for required dependencies
printf "Checking dependencies...\n"
check_command "curl"
check_command "tar"

printf "${GREEN}Installing vault-loader...${NC}\n"

# Detect system architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64)
        ARCH="arm64"
        ;;
    *)
        printf "${RED}Unsupported architecture: %s${NC}\n" "$ARCH"
        exit 1
        ;;
esac

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')

# Create temporary directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Download URL
DOWNLOAD_URL="https://vault-loader.idot-digital.com/vault-loader-${OS}-${ARCH}.tar.gz"

printf "Downloading vault-loader...\n"
if ! curl -L "$DOWNLOAD_URL" -o vault-loader.tar.gz; then
    printf "${RED}Failed to download vault-loader${NC}\n"
    exit 1
fi

# Extract the archive
tar xzf vault-loader.tar.gz

# Install binary
printf "Installing vault-loader binary...\n"
install -m 755 "vault-loader-${OS}-${ARCH}" /usr/local/bin/vault-loader

# Clean up
cd - > /dev/null
rm -rf "$TMP_DIR"

printf "${GREEN}vault-loader has been successfully installed!${NC}\n"
printf "You can now use the 'vault-loader' command from anywhere.\n" 