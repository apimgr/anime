#!/bin/bash
################################################################################
# Anime Quotes API - macOS Installation Script
# Supports: macOS (Intel and Apple Silicon)
# Repository: github.com/apimgr/anime
################################################################################

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
VERSION="${VERSION:-0.0.1}"
GITHUB_REPO="apimgr/anime"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/Library/Application Support/Anime"
DATA_DIR="/Library/Application Support/Anime/data"
LOG_DIR="/Library/Logs/Anime"
SERVICE_LABEL="com.apimgr.anime"
PLIST_PATH="/Library/LaunchDaemons/${SERVICE_LABEL}.plist"
INSTALL_LOG="/tmp/anime-install-$(date +%Y%m%d-%H%M%S).log"

# Logging functions
log() {
    echo -e "${GREEN}[INFO]${NC} $*" | tee -a "$INSTALL_LOG"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*" | tee -a "$INSTALL_LOG"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" | tee -a "$INSTALL_LOG"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $*" | tee -a "$INSTALL_LOG"
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Detect architecture
detect_arch() {
    local arch
    arch=$(uname -m)

    case "$arch" in
        x86_64)
            echo "amd64"
            ;;
        arm64)
            echo "arm64"
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
}

# Download binary from GitHub releases
download_binary() {
    local arch=$1
    local binary_name="anime-macos-${arch}"
    local download_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${binary_name}"

    log_step "Downloading Anime API binary..."
    log "Architecture: $arch ($(uname -m))"
    log "Version: $VERSION"
    log "URL: $download_url"

    if ! curl -sSL -o "/tmp/${binary_name}" "$download_url"; then
        log_error "Failed to download binary"
        exit 1
    fi

    chmod +x "/tmp/${binary_name}"
    log "Binary downloaded successfully"
}

# Install binary
install_binary() {
    local arch=$1
    local binary_name="anime-macos-${arch}"

    log_step "Installing binary to ${INSTALL_DIR}/anime..."

    if [ -f "${INSTALL_DIR}/anime" ]; then
        log_warn "Existing binary found. Creating backup..."
        mv "${INSTALL_DIR}/anime" "${INSTALL_DIR}/anime.bak.$(date +%s)"
    fi

    mv "/tmp/${binary_name}" "${INSTALL_DIR}/anime"
    chmod 755 "${INSTALL_DIR}/anime"

    # Remove quarantine attribute (macOS security)
    xattr -d com.apple.quarantine "${INSTALL_DIR}/anime" 2>/dev/null || true

    # Verify installation
    if "${INSTALL_DIR}/anime" --version >/dev/null 2>&1; then
        log "Binary installed and verified successfully"
    else
        log_warn "Binary installed but version check failed"
    fi
}

# Create directories
create_directories() {
    log_step "Creating application directories..."

    for dir in "$CONFIG_DIR" "$DATA_DIR" "$DATA_DIR/db" "$LOG_DIR"; do
        if [ ! -d "$dir" ]; then
            mkdir -p "$dir"
            log "Created directory: $dir"
        else
            log "Directory already exists: $dir"
        fi
    done

    # Set permissions (readable by all, writable by root)
    chmod 755 "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
    chmod 775 "$DATA_DIR/db"

    log "Permissions configured"
}

# Generate default admin credentials
generate_credentials() {
    local cred_file="${CONFIG_DIR}/admin_credentials"

    if [ -f "$cred_file" ]; then
        log "Admin credentials file already exists"
        return
    fi

    log_step "Generating default admin credentials..."

    local admin_user="administrator"
    local admin_pass=$(openssl rand -base64 16 | tr -d '/+=' | head -c 16)
    local admin_token=$(openssl rand -hex 32)

    cat > "$cred_file" <<EOF
# Anime API Admin Credentials
# Generated on $(date)
ADMIN_USER=$admin_user
ADMIN_PASSWORD=$admin_pass
ADMIN_TOKEN=$admin_token
EOF

    chmod 600 "$cred_file"

    log "Admin credentials generated: $cred_file"
    echo
    echo "=========================================="
    echo "DEFAULT ADMIN CREDENTIALS"
    echo "=========================================="
    echo "Username: $admin_user"
    echo "Password: $admin_pass"
    echo "Token:    $admin_token"
    echo "=========================================="
    echo "PLEASE CHANGE THESE IMMEDIATELY!"
    echo "=========================================="
    echo
}

# Create launchd plist
create_launchd_service() {
    log_step "Creating launchd service..."

    cat > "$PLIST_PATH" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${SERVICE_LABEL}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/anime</string>
        <string>--port</string>
        <string>8080</string>
        <string>--address</string>
        <string>::</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>

    <key>StandardOutPath</key>
    <string>${LOG_DIR}/anime.log</string>

    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/error.log</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>CONFIG_DIR</key>
        <string>${CONFIG_DIR}</string>
        <key>DATA_DIR</key>
        <string>${DATA_DIR}</string>
        <key>LOGS_DIR</key>
        <string>${LOG_DIR}</string>
    </dict>

    <key>WorkingDirectory</key>
    <string>${DATA_DIR}</string>

    <key>ProcessType</key>
    <string>Background</string>

    <key>Nice</key>
    <integer>0</integer>
</dict>
</plist>
EOF

    chmod 644 "$PLIST_PATH"
    log "Launchd plist created: $PLIST_PATH"
}

# Load and start service
load_service() {
    log_step "Loading and starting service..."

    # Unload if already loaded
    if launchctl list | grep -q "$SERVICE_LABEL"; then
        log "Unloading existing service..."
        launchctl unload "$PLIST_PATH" 2>/dev/null || true
    fi

    # Load service
    if launchctl load "$PLIST_PATH"; then
        log "Service loaded successfully"
        sleep 2

        # Check if service is running
        if launchctl list | grep -q "$SERVICE_LABEL"; then
            log "Service is running"
        else
            log_error "Service failed to start"
            return 1
        fi
    else
        log_error "Failed to load service"
        return 1
    fi
}

# Display access information
display_info() {
    log_step "Installation complete!"

    # Get local IP addresses
    local ipv4=$(ifconfig | grep "inet " | grep -v 127.0.0.1 | awk '{print $2}' | head -n 1)
    local ipv6=$(ifconfig | grep "inet6 " | grep -v "::1" | grep -v "fe80" | awk '{print $2}' | head -n 1)

    echo
    echo "=========================================="
    echo "INSTALLATION SUMMARY"
    echo "=========================================="
    echo "Binary:        ${INSTALL_DIR}/anime"
    echo "Config Dir:    ${CONFIG_DIR}"
    echo "Data Dir:      ${DATA_DIR}"
    echo "Log Dir:       ${LOG_DIR}"
    echo "Service:       ${SERVICE_LABEL}"
    echo "=========================================="
    echo
    echo "API ACCESS:"
    echo "  Local:       http://localhost:8080"
    if [ -n "$ipv4" ]; then
        echo "  IPv4:        http://${ipv4}:8080"
    fi
    if [ -n "$ipv6" ]; then
        echo "  IPv6:        http://[${ipv6}]:8080"
    fi
    echo
    echo "API ENDPOINTS:"
    echo "  Random Quote: GET /api/v1/random"
    echo "  All Quotes:   GET /api/v1/quotes"
    echo "  Health Check: GET /api/v1/health"
    echo "  Admin Panel:  http://localhost:8080/admin"
    echo
    echo "SERVICE MANAGEMENT:"
    echo "  Status:  sudo launchctl list | grep anime"
    echo "  Start:   sudo launchctl load ${PLIST_PATH}"
    echo "  Stop:    sudo launchctl unload ${PLIST_PATH}"
    echo "  Restart: sudo launchctl unload ${PLIST_PATH} && sudo launchctl load ${PLIST_PATH}"
    echo "  Logs:    tail -f ${LOG_DIR}/anime.log"
    echo
    echo "CREDENTIALS:"
    echo "  See: ${CONFIG_DIR}/admin_credentials"
    echo
    echo "=========================================="
    echo

    log "Installation log saved to: $INSTALL_LOG"
}

# Main installation flow
main() {
    log "Starting Anime API installation for macOS..."
    log "Installation log: $INSTALL_LOG"
    echo

    # Pre-flight checks
    check_root

    # Detect architecture
    ARCH=$(detect_arch)
    log "Detected architecture: $ARCH ($(uname -m))"
    echo

    # Prompt for confirmation
    read -p "Continue with installation? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log "Installation cancelled by user"
        exit 0
    fi

    # Installation steps
    download_binary "$ARCH"
    install_binary "$ARCH"
    create_directories
    generate_credentials
    create_launchd_service
    load_service

    # Display information
    display_info
}

# Run main function
main "$@"
