#!/bin/bash
################################################################################
# Anime Quotes API - Linux Installation Script
# Supports: All Linux distributions and init systems
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
CONFIG_DIR="/etc/anime"
DATA_DIR="/var/lib/anime"
LOG_DIR="/var/log/anime"
SERVICE_USER="anime"
SERVICE_GROUP="anime"
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

# Detect operating system
detect_os() {
    log_step "Detecting operating system..."

    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS_NAME=$ID
        OS_VERSION=$VERSION_ID
        OS_PRETTY=$PRETTY_NAME
    elif [ -f /etc/lsb-release ]; then
        . /etc/lsb-release
        OS_NAME=$DISTRIB_ID
        OS_VERSION=$DISTRIB_RELEASE
        OS_PRETTY="$DISTRIB_ID $DISTRIB_RELEASE"
    else
        OS_NAME=$(uname -s)
        OS_VERSION=$(uname -r)
        OS_PRETTY="$OS_NAME $OS_VERSION"
    fi

    log "Detected OS: $OS_PRETTY"
    echo "$OS_NAME" | tr '[:upper:]' '[:lower:]'
}

# Detect init system
detect_init_system() {
    log_step "Detecting init system..."

    if [ -d /run/systemd/system ]; then
        echo "systemd"
    elif [ -f /sbin/openrc-run ] || [ -d /etc/init.d ] && grep -q "openrc" /sbin/init 2>/dev/null; then
        echo "openrc"
    elif [ -d /etc/runit ]; then
        echo "runit"
    elif [ -d /etc/s6 ]; then
        echo "s6"
    elif [ -f /sbin/init ] && strings /sbin/init | grep -q "sysvinit"; then
        echo "sysvinit"
    else
        echo "unknown"
    fi
}

# Detect architecture
detect_arch() {
    local arch
    arch=$(uname -m)

    case "$arch" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
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
    local binary_name="anime-linux-${arch}"
    local download_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${binary_name}"

    log_step "Downloading Anime API binary..."
    log "Architecture: $arch"
    log "Version: $VERSION"
    log "URL: $download_url"

    if command -v curl >/dev/null 2>&1; then
        curl -sSL -o "/tmp/${binary_name}" "$download_url" || {
            log_error "Failed to download binary"
            exit 1
        }
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O "/tmp/${binary_name}" "$download_url" || {
            log_error "Failed to download binary"
            exit 1
        }
    else
        log_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi

    chmod +x "/tmp/${binary_name}"
    log "Binary downloaded successfully"
}

# Install binary
install_binary() {
    local arch=$1
    local binary_name="anime-linux-${arch}"

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

    if id "$SERVICE_USER" >/dev/null 2>&1; then
        log "User '$SERVICE_USER' already exists"
    else
        case "$OS_NAME" in
            alpine)
                addgroup -S "$SERVICE_GROUP" 2>/dev/null || true
                adduser -S -D -H -h /nonexistent -s /sbin/nologin -G "$SERVICE_GROUP" "$SERVICE_USER"
                ;;
            *)
                groupadd -r "$SERVICE_GROUP" 2>/dev/null || true
                useradd -r -s /bin/false -g "$SERVICE_GROUP" -d /nonexistent -c "Anime API Service" "$SERVICE_USER"
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

# Create systemd service
create_systemd_service() {
    log_step "Creating systemd service..."

    cat > /etc/systemd/system/anime.service <<'EOF'
[Unit]
Description=Anime Quotes API Service
Documentation=https://github.com/apimgr/anime
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=anime
Group=anime
ExecStart=/usr/local/bin/anime --port 8080 --address ::
Restart=on-failure
RestartSec=5
StandardOutput=append:/var/log/anime/anime.log
StandardError=append:/var/log/anime/error.log

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/anime /var/log/anime
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# Environment
Environment="CONFIG_DIR=/etc/anime"
Environment="DATA_DIR=/var/lib/anime"
Environment="LOGS_DIR=/var/log/anime"

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    log "Systemd service created: anime.service"
}

# Create OpenRC service
create_openrc_service() {
    log_step "Creating OpenRC service..."

    cat > /etc/init.d/anime <<'EOF'
#!/sbin/openrc-run

name="anime"
description="Anime Quotes API Service"
command="/usr/local/bin/anime"
command_args="--port 8080 --address ::"
command_user="anime:anime"
pidfile="/run/${RC_SVCNAME}.pid"
command_background="yes"
output_log="/var/log/anime/anime.log"
error_log="/var/log/anime/error.log"

depend() {
    need net
    after firewall
}

start_pre() {
    checkpath --directory --owner anime:anime --mode 0750 /var/lib/anime
    checkpath --directory --owner anime:anime --mode 0750 /var/log/anime
}
EOF

    chmod 755 /etc/init.d/anime
    log "OpenRC service created: /etc/init.d/anime"
}

# Create runit service
create_runit_service() {
    log_step "Creating runit service..."

    local service_dir="/etc/sv/anime"
    mkdir -p "$service_dir/log"

    cat > "$service_dir/run" <<'EOF'
#!/bin/sh
exec 2>&1
exec chpst -u anime:anime /usr/local/bin/anime --port 8080 --address ::
EOF

    cat > "$service_dir/log/run" <<'EOF'
#!/bin/sh
exec svlogd -tt /var/log/anime
EOF

    chmod 755 "$service_dir/run"
    chmod 755 "$service_dir/log/run"

    log "Runit service created: $service_dir"
}

# Create s6 service
create_s6_service() {
    log_step "Creating s6 service..."

    local service_dir="/etc/s6/sv/anime"
    mkdir -p "$service_dir"

    cat > "$service_dir/run" <<'EOF'
#!/bin/execlineb -P
s6-setuidgid anime
/usr/local/bin/anime --port 8080 --address ::
EOF

    chmod 755 "$service_dir/run"

    log "s6 service created: $service_dir"
}

# Create sysvinit service
create_sysvinit_service() {
    log_step "Creating sysvinit service..."

    cat > /etc/init.d/anime <<'EOF'
#!/bin/bash
### BEGIN INIT INFO
# Provides:          anime
# Required-Start:    $network $remote_fs $syslog
# Required-Stop:     $network $remote_fs $syslog
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: Anime Quotes API Service
# Description:       Anime Quotes REST API Server
### END INIT INFO

DAEMON=/usr/local/bin/anime
NAME=anime
PIDFILE=/var/run/$NAME.pid
USER=anime
GROUP=anime

start() {
    echo "Starting $NAME..."
    start-stop-daemon --start --quiet --pidfile $PIDFILE \
        --make-pidfile --background --chuid $USER:$GROUP \
        --exec $DAEMON -- --port 8080 --address ::
    echo "$NAME started."
}

stop() {
    echo "Stopping $NAME..."
    start-stop-daemon --stop --quiet --pidfile $PIDFILE
    rm -f $PIDFILE
    echo "$NAME stopped."
}

case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        stop
        sleep 2
        start
        ;;
    *)
        echo "Usage: $0 {start|stop|restart}"
        exit 1
        ;;
esac

exit 0
EOF

    chmod 755 /etc/init.d/anime

    # Try to add to default runlevels
    if command -v update-rc.d >/dev/null 2>&1; then
        update-rc.d anime defaults
    elif command -v chkconfig >/dev/null 2>&1; then
        chkconfig --add anime
        chkconfig anime on
    fi

    log "Sysvinit service created: /etc/init.d/anime"
}

# Enable and start service
enable_and_start_service() {
    local init_system=$1

    log_step "Enabling and starting service..."

    case "$init_system" in
        systemd)
            systemctl enable anime.service
            systemctl start anime.service

            if systemctl is-active --quiet anime.service; then
                log "Service started successfully"
            else
                log_error "Service failed to start. Check logs: journalctl -u anime"
                return 1
            fi
            ;;
        openrc)
            rc-update add anime default
            rc-service anime start

            if rc-service anime status >/dev/null 2>&1; then
                log "Service started successfully"
            else
                log_error "Service failed to start. Check logs: /var/log/anime/error.log"
                return 1
            fi
            ;;
        runit)
            ln -sf /etc/sv/anime /var/service/
            sleep 2

            if sv status anime >/dev/null 2>&1; then
                log "Service started successfully"
            else
                log_error "Service failed to start. Check logs: /var/log/anime/current"
                return 1
            fi
            ;;
        s6)
            s6-rc-bundle-update add default anime
            s6-rc -u change anime
            log "Service started successfully"
            ;;
        sysvinit)
            /etc/init.d/anime start
            log "Service started"
            ;;
        *)
            log_warn "Unknown init system. Please start the service manually."
            return 1
            ;;
    esac
}

# Display access information
display_info() {
    log_step "Installation complete!"

    echo
    echo "=========================================="
    echo "INSTALLATION SUMMARY"
    echo "=========================================="
    echo "Binary:        ${INSTALL_DIR}/anime"
    echo "Config Dir:    ${CONFIG_DIR}"
    echo "Data Dir:      ${DATA_DIR}"
    echo "Log Dir:       ${LOG_DIR}"
    echo "Service User:  ${SERVICE_USER}"
    echo "=========================================="
    echo
    echo "API ACCESS:"
    echo "  Local:       http://localhost:8080"
    echo "  IPv4:        http://$(hostname -I | awk '{print $1}'):8080"
    echo "  IPv6:        http://[$(hostname -I | awk '{print $2}')]:8080"
    echo
    echo "API ENDPOINTS:"
    echo "  Random Quote: GET /api/v1/random"
    echo "  All Quotes:   GET /api/v1/quotes"
    echo "  Health Check: GET /api/v1/health"
    echo "  Admin Panel:  http://localhost:8080/admin"
    echo
    echo "SERVICE MANAGEMENT:"

    local init_system=$(detect_init_system)
    case "$init_system" in
        systemd)
            echo "  Status:  systemctl status anime"
            echo "  Start:   systemctl start anime"
            echo "  Stop:    systemctl stop anime"
            echo "  Restart: systemctl restart anime"
            echo "  Logs:    journalctl -u anime -f"
            ;;
        openrc)
            echo "  Status:  rc-service anime status"
            echo "  Start:   rc-service anime start"
            echo "  Stop:    rc-service anime stop"
            echo "  Restart: rc-service anime restart"
            echo "  Logs:    tail -f /var/log/anime/anime.log"
            ;;
        runit)
            echo "  Status:  sv status anime"
            echo "  Start:   sv up anime"
            echo "  Stop:    sv down anime"
            echo "  Restart: sv restart anime"
            echo "  Logs:    tail -f /var/log/anime/current"
            ;;
        *)
            echo "  Check your init system documentation"
            ;;
    esac

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
    log "Starting Anime API installation..."
    log "Installation log: $INSTALL_LOG"
    echo

    # Pre-flight checks
    check_root

    # Detect system
    OS_NAME=$(detect_os)
    INIT_SYSTEM=$(detect_init_system)
    ARCH=$(detect_arch)

    log "OS: $OS_NAME"
    log "Init System: $INIT_SYSTEM"
    log "Architecture: $ARCH"
    echo

    # Prompt for confirmation
    read -p "Continue with installation? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log "Installation cancelled by user"
        exit 0
    fi

    # Install dependencies
    log_step "Installing dependencies..."
    case "$OS_NAME" in
        ubuntu|debian)
            apt-get update -qq
            apt-get install -y -qq curl ca-certificates openssl > /dev/null
            ;;
        fedora|rhel|centos)
            dnf install -y -q curl ca-certificates openssl > /dev/null
            ;;
        alpine)
            apk add --no-cache curl ca-certificates openssl > /dev/null
            ;;
        arch)
            pacman -Sy --noconfirm curl ca-certificates openssl > /dev/null
            ;;
    esac

    # Installation steps
    download_binary "$ARCH"
    install_binary "$ARCH"
    create_user
    create_directories
    generate_credentials

    # Create service based on init system
    case "$INIT_SYSTEM" in
        systemd)
            create_systemd_service
            ;;
        openrc)
            create_openrc_service
            ;;
        runit)
            create_runit_service
            ;;
        s6)
            create_s6_service
            ;;
        sysvinit)
            create_sysvinit_service
            ;;
        *)
            log_warn "Unknown init system. Service not created."
            log "Please create service manually."
            ;;
    esac

    # Enable and start service
    if [ "$INIT_SYSTEM" != "unknown" ]; then
        enable_and_start_service "$INIT_SYSTEM"
    fi

    # Display information
    display_info
}

# Run main function
main "$@"
