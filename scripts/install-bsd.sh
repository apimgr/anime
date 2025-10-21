#!/bin/sh
################################################################################
# Anime Quotes API - BSD Installation Script
# Supports: FreeBSD, OpenBSD, NetBSD
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
CONFIG_DIR="/usr/local/etc/anime"
DATA_DIR="/var/db/anime"
LOG_DIR="/var/log/anime"
SERVICE_USER="anime"
SERVICE_GROUP="anime"
INSTALL_LOG="/tmp/anime-install-$(date +%Y%m%d-%H%M%S).log"

# Logging functions
log() {
    printf "${GREEN}[INFO]${NC} %s\n" "$*" | tee -a "$INSTALL_LOG"
}

log_warn() {
    printf "${YELLOW}[WARN]${NC} %s\n" "$*" | tee -a "$INSTALL_LOG"
}

log_error() {
    printf "${RED}[ERROR]${NC} %s\n" "$*" | tee -a "$INSTALL_LOG"
}

log_step() {
    printf "${BLUE}[STEP]${NC} %s\n" "$*" | tee -a "$INSTALL_LOG"
}

# Check if running as root
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

# Detect BSD variant
detect_bsd() {
    log_step "Detecting BSD variant..."

    local os_name
    os_name=$(uname -s)

    case "$os_name" in
        FreeBSD)
            echo "freebsd"
            ;;
        OpenBSD)
            echo "openbsd"
            ;;
        NetBSD)
            echo "netbsd"
            ;;
        *)
            log_error "Unsupported BSD variant: $os_name"
            exit 1
            ;;
    esac
}

# Detect architecture
detect_arch() {
    local arch
    arch=$(uname -m)

    case "$arch" in
        amd64|x86_64)
            echo "amd64"
            ;;
        arm64|aarch64)
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
    # Note: We use FreeBSD binary for all BSD variants (same ABI)
    local binary_name="anime-freebsd-${arch}"
    local download_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${binary_name}"

    log_step "Downloading Anime API binary..."
    log "Architecture: $arch"
    log "Version: $VERSION"
    log "URL: $download_url"

    if command -v fetch >/dev/null 2>&1; then
        # FreeBSD fetch
        fetch -o "/tmp/${binary_name}" "$download_url" || {
            log_error "Failed to download binary"
            exit 1
        }
    elif command -v ftp >/dev/null 2>&1; then
        # OpenBSD/NetBSD ftp
        ftp -o "/tmp/${binary_name}" "$download_url" || {
            log_error "Failed to download binary"
            exit 1
        }
    elif command -v curl >/dev/null 2>&1; then
        curl -sSL -o "/tmp/${binary_name}" "$download_url" || {
            log_error "Failed to download binary"
            exit 1
        }
    else
        log_error "No download utility found (fetch, ftp, or curl)"
        exit 1
    fi

    chmod +x "/tmp/${binary_name}"
    log "Binary downloaded successfully"
}

# Install binary
install_binary() {
    local arch=$1
    local binary_name="anime-freebsd-${arch}"

    log_step "Installing binary to ${INSTALL_DIR}/anime..."

    if [ -f "${INSTALL_DIR}/anime" ]; then
        log_warn "Existing binary found. Creating backup..."
        mv "${INSTALL_DIR}/anime" "${INSTALL_DIR}/anime.bak.$(date +%s)"
    fi

    mv "/tmp/${binary_name}" "${INSTALL_DIR}/anime"
    chmod 755 "${INSTALL_DIR}/anime"

    # Verify installation
    if "${INSTALL_DIR}/anime" --version >/dev/null 2>&1; then
        log "Binary installed and verified successfully"
    else
        log_warn "Binary installed but version check failed"
    fi
}

# Create system user
create_user() {
    log_step "Creating system user and group..."

    local bsd_variant=$1

    if id "$SERVICE_USER" >/dev/null 2>&1; then
        log "User '$SERVICE_USER' already exists"
    else
        case "$bsd_variant" in
            freebsd)
                pw groupadd "$SERVICE_GROUP" 2>/dev/null || true
                pw useradd "$SERVICE_USER" -g "$SERVICE_GROUP" -s /usr/sbin/nologin -d /nonexistent -c "Anime API Service"
                ;;
            openbsd|netbsd)
                groupadd "$SERVICE_GROUP" 2>/dev/null || true
                useradd -g "$SERVICE_GROUP" -s /sbin/nologin -d /nonexistent -c "Anime API Service" "$SERVICE_USER"
                ;;
        esac
        log "Created user: $SERVICE_USER"
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

    # Set permissions
    chown -R "$SERVICE_USER:$SERVICE_GROUP" "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
    chmod 750 "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
    chmod 770 "$DATA_DIR/db"

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
    chown "$SERVICE_USER:$SERVICE_GROUP" "$cred_file"

    log "Admin credentials generated: $cred_file"
    printf "\n"
    printf "==========================================\n"
    printf "DEFAULT ADMIN CREDENTIALS\n"
    printf "==========================================\n"
    printf "Username: %s\n" "$admin_user"
    printf "Password: %s\n" "$admin_pass"
    printf "Token:    %s\n" "$admin_token"
    printf "==========================================\n"
    printf "PLEASE CHANGE THESE IMMEDIATELY!\n"
    printf "==========================================\n"
    printf "\n"
}

# Create rc.d script
create_rc_script() {
    local bsd_variant=$1

    log_step "Creating rc.d service script..."

    cat > /usr/local/etc/rc.d/anime <<'EOF'
#!/bin/sh

# PROVIDE: anime
# REQUIRE: NETWORKING DAEMON
# KEYWORD: shutdown

. /etc/rc.subr

name="anime"
rcvar=anime_enable

load_rc_config $name

: ${anime_enable:="NO"}
: ${anime_user:="anime"}
: ${anime_group:="anime"}
: ${anime_port:="8080"}
: ${anime_address:="::"}

command="/usr/local/bin/anime"
command_args="--port ${anime_port} --address ${anime_address}"
pidfile="/var/run/${name}.pid"

start_cmd="${name}_start"
stop_cmd="${name}_stop"
status_cmd="${name}_status"

anime_start() {
    echo "Starting ${name}."
    /usr/sbin/daemon -p ${pidfile} -u ${anime_user} \
        ${command} ${command_args} \
        >> /var/log/anime/anime.log 2>> /var/log/anime/error.log
}

anime_stop() {
    if [ -f ${pidfile} ]; then
        echo "Stopping ${name}."
        kill -TERM $(cat ${pidfile})
        rm -f ${pidfile}
    else
        echo "${name} is not running."
    fi
}

anime_status() {
    if [ -f ${pidfile} ]; then
        pid=$(cat ${pidfile})
        if ps -p ${pid} > /dev/null 2>&1; then
            echo "${name} is running as pid ${pid}."
        else
            echo "${name} is not running but pidfile exists."
        fi
    else
        echo "${name} is not running."
    fi
}

run_rc_command "$1"
EOF

    chmod 755 /usr/local/etc/rc.d/anime
    log "rc.d script created: /usr/local/etc/rc.d/anime"
}

# Enable and start service
enable_and_start_service() {
    local bsd_variant=$1

    log_step "Enabling and starting service..."

    # Add to rc.conf
    if ! grep -q "anime_enable=" /etc/rc.conf 2>/dev/null; then
        echo 'anime_enable="YES"' >> /etc/rc.conf
        log "Added anime_enable to /etc/rc.conf"
    fi

    # Start service
    case "$bsd_variant" in
        freebsd)
            service anime start
            if service anime status >/dev/null 2>&1; then
                log "Service started successfully"
            else
                log_error "Service failed to start. Check logs: /var/log/anime/error.log"
                return 1
            fi
            ;;
        openbsd|netbsd)
            /usr/local/etc/rc.d/anime start
            log "Service started"
            ;;
    esac
}

# Display access information
display_info() {
    local bsd_variant=$1

    log_step "Installation complete!"

    # Get local IP addresses
    local ipv4=$(ifconfig | grep "inet " | grep -v 127.0.0.1 | awk '{print $2}' | head -n 1)
    local ipv6=$(ifconfig | grep "inet6 " | grep -v "::1" | grep -v "fe80" | awk '{print $2}' | head -n 1)

    printf "\n"
    printf "==========================================\n"
    printf "INSTALLATION SUMMARY\n"
    printf "==========================================\n"
    printf "Binary:        %s\n" "${INSTALL_DIR}/anime"
    printf "Config Dir:    %s\n" "$CONFIG_DIR"
    printf "Data Dir:      %s\n" "$DATA_DIR"
    printf "Log Dir:       %s\n" "$LOG_DIR"
    printf "Service User:  %s\n" "$SERVICE_USER"
    printf "BSD Variant:   %s\n" "$bsd_variant"
    printf "==========================================\n"
    printf "\n"
    printf "API ACCESS:\n"
    printf "  Local:       http://localhost:8080\n"
    [ -n "$ipv4" ] && printf "  IPv4:        http://%s:8080\n" "$ipv4"
    [ -n "$ipv6" ] && printf "  IPv6:        http://[%s]:8080\n" "$ipv6"
    printf "\n"
    printf "API ENDPOINTS:\n"
    printf "  Random Quote: GET /api/v1/random\n"
    printf "  All Quotes:   GET /api/v1/quotes\n"
    printf "  Health Check: GET /api/v1/health\n"
    printf "  Admin Panel:  http://localhost:8080/admin\n"
    printf "\n"
    printf "SERVICE MANAGEMENT:\n"

    case "$bsd_variant" in
        freebsd)
            printf "  Status:  service anime status\n"
            printf "  Start:   service anime start\n"
            printf "  Stop:    service anime stop\n"
            printf "  Restart: service anime restart\n"
            ;;
        openbsd|netbsd)
            printf "  Status:  /usr/local/etc/rc.d/anime status\n"
            printf "  Start:   /usr/local/etc/rc.d/anime start\n"
            printf "  Stop:    /usr/local/etc/rc.d/anime stop\n"
            printf "  Restart: /usr/local/etc/rc.d/anime restart\n"
            ;;
    esac

    printf "  Logs:    tail -f %s/anime.log\n" "$LOG_DIR"
    printf "\n"
    printf "CREDENTIALS:\n"
    printf "  See: %s/admin_credentials\n" "$CONFIG_DIR"
    printf "\n"
    printf "==========================================\n"
    printf "\n"

    log "Installation log saved to: $INSTALL_LOG"
}

# Main installation flow
main() {
    log "Starting Anime API installation for BSD..."
    log "Installation log: $INSTALL_LOG"
    printf "\n"

    # Pre-flight checks
    check_root

    # Detect system
    BSD_VARIANT=$(detect_bsd)
    ARCH=$(detect_arch)

    log "BSD Variant: $BSD_VARIANT ($(uname -s) $(uname -r))"
    log "Architecture: $ARCH"
    printf "\n"

    # Prompt for confirmation
    printf "Continue with installation? (y/N): "
    read -r reply
    case "$reply" in
        [Yy]*)
            ;;
        *)
            log "Installation cancelled by user"
            exit 0
            ;;
    esac

    # Install dependencies
    log_step "Installing dependencies..."
    case "$BSD_VARIANT" in
        freebsd)
            pkg install -y curl ca_root_nss openssl >/dev/null 2>&1 || true
            ;;
        openbsd)
            pkg_add curl >/dev/null 2>&1 || true
            ;;
        netbsd)
            pkgin -y install curl mozilla-rootcerts openssl >/dev/null 2>&1 || true
            ;;
    esac

    # Installation steps
    download_binary "$ARCH"
    install_binary "$ARCH"
    create_user "$BSD_VARIANT"
    create_directories
    generate_credentials
    create_rc_script "$BSD_VARIANT"
    enable_and_start_service "$BSD_VARIANT"

    # Display information
    display_info "$BSD_VARIANT"
}

# Run main function
main "$@"
