#!/bin/sh
set -e

REPO="fastclaw-ai/anyclaw"
BINARY="anyclaw"
# Install dir: user-owned by default, no sudo needed
# Override with: ANYCLAW_INSTALL_DIR=/usr/local/bin curl ... | sh
INSTALL_DIR="${ANYCLAW_INSTALL_DIR:-$HOME/.local/bin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()    { printf "${BLUE}[anyclaw]${NC} %s\n" "$1" >&2; }
success() { printf "${GREEN}[anyclaw]${NC} %s\n" "$1" >&2; }
warn()    { printf "${YELLOW}[anyclaw]${NC} %s\n" "$1" >&2; }
error()   { printf "${RED}[anyclaw]${NC} ERROR: %s\n" "$1" >&2; exit 1; }

# Detect OS and architecture
detect_platform() {
    OS="$(uname -s 2>/dev/null || echo unknown)"
    ARCH="$(uname -m 2>/dev/null || echo unknown)"

    case "$OS" in
        Linux)  OS="linux" ;;
        Darwin) OS="darwin" ;;
        MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
        *) error "Unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        armv7l|armv6l) ARCH="arm" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    PLATFORM="${OS}_${ARCH}"
}

# Fetch latest release tag from GitHub
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
            | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" \
            | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
    else
        error "curl or wget is required"
    fi

    [ -n "$VERSION" ] || error "Could not fetch latest version. Check: https://github.com/${REPO}/releases"
}

# Download the binary for current platform, echo the temp path
download_binary() {
    FILENAME="${BINARY}_${PLATFORM}.tar.gz"
    # Windows uses .zip
    [ "$OS" = "windows" ] && FILENAME="${BINARY}_${PLATFORM}.zip"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

    TMP_DIR="$(mktemp -d)"
    TMP_ARCHIVE="${TMP_DIR}/${FILENAME}"

    info "Downloading ${BINARY} ${VERSION} for ${PLATFORM}..."

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$URL" -o "$TMP_ARCHIVE" \
            || error "Download failed. Check: https://github.com/${REPO}/releases"
    else
        wget -qO "$TMP_ARCHIVE" "$URL" \
            || error "Download failed. Check: https://github.com/${REPO}/releases"
    fi

    # Extract
    if [ "$OS" = "windows" ]; then
        unzip -q "$TMP_ARCHIVE" -d "$TMP_DIR"
        TMP_BIN="${TMP_DIR}/${BINARY}.exe"
    else
        tar xzf "$TMP_ARCHIVE" -C "$TMP_DIR"
        TMP_BIN="${TMP_DIR}/${BINARY}"
    fi

    chmod +x "$TMP_BIN"
    printf "%s" "$TMP_BIN"
}

# Install binary to INSTALL_DIR (no sudo)
install_binary() {
    TMP_BIN="$1"
    mkdir -p "$INSTALL_DIR"

    # Atomic replace: backup → move → verify → remove backup
    DEST="${INSTALL_DIR}/${BINARY}"
    if [ -f "$DEST" ]; then
        cp "$DEST" "${DEST}.bak" 2>/dev/null || true
    fi

    mv "$TMP_BIN" "$DEST"

    # Verify the new binary works; restore backup on failure
    if ! "$DEST" version >/dev/null 2>&1; then
        [ -f "${DEST}.bak" ] && mv "${DEST}.bak" "$DEST"
        error "Installed binary failed to run. Restored previous version."
    fi

    rm -f "${DEST}.bak"
    success "Installed to ${DEST}"
}

# Detect the current shell's config file
detect_shell_config() {
    SHELL_NAME="$(basename "${SHELL:-sh}")"
    case "$SHELL_NAME" in
        zsh)  printf "%s" "$HOME/.zshrc" ;;
        bash)
            if [ "$(uname -s)" = "Darwin" ]; then
                printf "%s" "$HOME/.bash_profile"
            else
                printf "%s" "$HOME/.bashrc"
            fi
            ;;
        fish) printf "%s" "$HOME/.config/fish/config.fish" ;;
        *)    printf "%s" "$HOME/.profile" ;;
    esac
}

# Add INSTALL_DIR to PATH in shell config (idempotent)
add_to_path() {
    # Already in current PATH? Nothing to do.
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) return 0 ;;
    esac

    SHELL_CONFIG="$(detect_shell_config)"

    if [ "$(basename "${SHELL:-sh}")" = "fish" ]; then
        PATH_LINE="fish_add_path ${INSTALL_DIR}"
    else
        PATH_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""
    fi

    # Already written in config?
    if [ -f "$SHELL_CONFIG" ] && grep -qF "$INSTALL_DIR" "$SHELL_CONFIG" 2>/dev/null; then
        return 0
    fi

    {
        printf "\n# Added by anyclaw installer\n"
        printf "%s\n" "$PATH_LINE"
    } >> "$SHELL_CONFIG"

    success "Added ${INSTALL_DIR} to PATH in ${SHELL_CONFIG}"
    NEED_SOURCE="$SHELL_CONFIG"
}

main() {
    printf "\n" >&2
    info "Installing anyclaw — the universal tool adapter for AI agents"
    printf "\n" >&2

    detect_platform
    info "Platform:    ${PLATFORM}"
    info "Install dir: ${INSTALL_DIR}"

    get_latest_version
    info "Version:     ${VERSION}"
    printf "\n" >&2

    TMP_BIN="$(download_binary)"
    install_binary "$TMP_BIN"
    add_to_path

    printf "\n" >&2
    success "🎉 anyclaw ${VERSION} installed!"
    printf "\n" >&2

    if [ -n "${NEED_SOURCE:-}" ]; then
        printf "  Activate PATH for this session:\n" >&2
        printf "    ${YELLOW}source ${NEED_SOURCE}${NC}\n" >&2
        printf "\n" >&2
    fi

    printf "  Quick start:\n" >&2
    printf "    ${GREEN}anyclaw search news${NC}              # discover packages\n" >&2
    printf "    ${GREEN}anyclaw install hackernews${NC}       # install a package\n" >&2
    printf "    ${GREEN}anyclaw install opencli/weibo${NC}    # install from a repo\n" >&2
    printf "    ${GREEN}anyclaw site update${NC}              # pull 98+ website adapters\n" >&2
    printf "    ${GREEN}anyclaw mcp${NC}                      # expose as MCP server\n" >&2
    printf "\n" >&2
    printf "  Self-update anytime:\n" >&2
    printf "    ${GREEN}anyclaw update${NC}\n" >&2
    printf "\n" >&2
}

main
