#!/usr/bin/env bash
#
# strands installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/Pinnacle-Solutions-Group/strands/main/scripts/install.sh | bash
#
# Downloads the latest prebuilt strands binary from GitHub releases,
# verifies its SHA256 checksum against checksums.txt, and installs it
# to /usr/local/bin (if writable) or $HOME/.local/bin.
#
# IMPORTANT: execute, do not source. ❌ source install.sh  ✅ bash install.sh
#

set -e

REPO="Pinnacle-Solutions-Group/strands"
BIN_NAME="strands"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()    { printf '%b==>%b %s\n' "$BLUE"   "$NC" "$1" >&2; }
log_success() { printf '%b==>%b %s\n' "$GREEN"  "$NC" "$1" >&2; }
log_warning() { printf '%b==>%b %s\n' "$YELLOW" "$NC" "$1" >&2; }
log_error()   { printf '%bError:%b %s\n' "$RED" "$NC" "$1" >&2; }

detect_platform() {
    local os arch
    case "$(uname -s)" in
        MINGW*|MSYS*|CYGWIN*)
            log_error "Windows is not supported by this installer."
            echo "  Install with: go install github.com/${REPO}/cmd/${BIN_NAME}@latest" >&2
            exit 1
            ;;
        Darwin) os="darwin" ;;
        Linux)  os="linux"  ;;
        *)
            log_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)  arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)
            log_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac

    printf '%s_%s\n' "$os" "$arch"
}

download_file() {
    local url=$1 output=$2
    if command -v curl &>/dev/null; then
        curl -fsSL -o "$output" "$url"
        return $?
    fi
    if command -v wget &>/dev/null; then
        wget -q -O "$output" "$url"
        return $?
    fi
    log_error "Neither curl nor wget found. Install one and retry."
    return 1
}

fetch_stdout() {
    local url=$1
    if command -v curl &>/dev/null; then
        curl -fsSL "$url"
        return $?
    fi
    if command -v wget &>/dev/null; then
        wget -qO- "$url"
        return $?
    fi
    log_error "Neither curl nor wget found. Install one and retry."
    return 1
}

sha256_file() {
    local f=$1
    if command -v sha256sum &>/dev/null; then
        sha256sum "$f" | awk '{print $1}'
        return 0
    fi
    if command -v shasum &>/dev/null; then
        shasum -a 256 "$f" | awk '{print $1}'
        return 0
    fi
    if command -v openssl &>/dev/null; then
        openssl dgst -sha256 "$f" | awk '{print $2}'
        return 0
    fi
    return 1
}

verify_checksum() {
    local archive=$1 checksums=$2
    local expected actual
    expected=$(awk -v t="$archive" '{n=$2; sub(/^\*/,"",n); if(n==t){print $1; exit}}' "$checksums")
    if [ -z "$expected" ]; then
        log_error "No checksum entry for $archive in $checksums"
        return 1
    fi
    if ! actual=$(sha256_file "$archive"); then
        log_error "No SHA256 tool found (need sha256sum, shasum, or openssl)"
        return 1
    fi
    if [ "$expected" != "$actual" ]; then
        log_error "Checksum mismatch for $archive"
        echo "  expected: $expected" >&2
        echo "  actual:   $actual"   >&2
        return 1
    fi
    log_success "Checksum verified for $archive"
}

install_from_release() {
    local platform=$1
    local tmp_dir
    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' RETURN

    log_info "Fetching latest release metadata..."
    local release_json version
    if ! release_json=$(fetch_stdout "https://api.github.com/repos/${REPO}/releases/latest"); then
        log_error "Failed to fetch release metadata from GitHub"
        return 1
    fi

    version=$(printf '%s' "$release_json" | grep '"tag_name"' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        log_error "Failed to parse tag_name from GitHub release metadata"
        return 1
    fi
    log_info "Latest version: $version"

    local archive="${BIN_NAME}_${version#v}_${platform}.tar.gz"
    local base_url="https://github.com/${REPO}/releases/download/${version}"

    cd "$tmp_dir"

    log_info "Downloading $archive..."
    if ! download_file "${base_url}/${archive}" "$archive"; then
        log_error "Failed to download ${base_url}/${archive}"
        return 1
    fi

    log_info "Downloading checksums.txt..."
    if ! download_file "${base_url}/checksums.txt" "checksums.txt"; then
        log_error "Failed to download checksums.txt"
        return 1
    fi

    verify_checksum "$archive" "checksums.txt" || return 1

    log_info "Extracting archive..."
    if ! tar -xzf "$archive"; then
        log_error "Failed to extract $archive"
        return 1
    fi

    local extracted
    extracted=$(find . -type f -name "$BIN_NAME" -perm -u+x 2>/dev/null | head -n1)
    if [ -z "$extracted" ]; then
        extracted=$(find . -type f -name "$BIN_NAME" 2>/dev/null | head -n1)
    fi
    if [ -z "$extracted" ]; then
        log_error "Extracted archive does not contain a '$BIN_NAME' binary"
        return 1
    fi
    chmod +x "$extracted"

    local install_dir
    if [ -w /usr/local/bin ]; then
        install_dir="/usr/local/bin"
    else
        install_dir="$HOME/.local/bin"
        mkdir -p "$install_dir"
    fi

    log_info "Installing to $install_dir..."
    if [ -w "$install_dir" ]; then
        mv "$extracted" "$install_dir/$BIN_NAME"
    else
        sudo mv "$extracted" "$install_dir/$BIN_NAME"
    fi

    LAST_INSTALL_PATH="$install_dir/$BIN_NAME"
    log_success "$BIN_NAME installed to $LAST_INSTALL_PATH"

    if [[ ":$PATH:" != *":$install_dir:"* ]]; then
        log_warning "$install_dir is not in your PATH"
        echo "" >&2
        echo "  Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):" >&2
        echo "    export PATH=\"\$PATH:$install_dir\"" >&2
        echo "" >&2
    fi
}

verify_installation() {
    if command -v "$BIN_NAME" &>/dev/null; then
        log_success "$BIN_NAME is installed and on your PATH"
        echo ""
        echo "Get started:"
        echo "  cd your-project"
        echo "  strands init"
        echo "  strands install-hook   # global Claude Code SessionStart hook"
        echo ""
        return 0
    fi

    if [ -n "${LAST_INSTALL_PATH:-}" ] && [ -x "$LAST_INSTALL_PATH" ]; then
        log_warning "$BIN_NAME installed to $LAST_INSTALL_PATH but is not yet on PATH"
        echo "  Open a new shell or add its directory to PATH, then run 'strands init'."
        return 0
    fi

    log_error "$BIN_NAME installation could not be verified"
    return 1
}

main() {
    echo ""
    echo "🪢 strands installer"
    echo ""

    log_info "Detecting platform..."
    local platform
    platform=$(detect_platform)
    log_info "Platform: $platform"

    if install_from_release "$platform"; then
        verify_installation
        exit 0
    fi

    log_error "Installation failed"
    echo ""
    echo "Fallback: go install github.com/${REPO}/cmd/${BIN_NAME}@latest"
    echo "  Make sure \$GOBIN (or \$(go env GOPATH)/bin) is on your PATH."
    echo ""
    exit 1
}

main "$@"
