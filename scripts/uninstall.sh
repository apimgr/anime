#!/bin/bash
################################################################################
# Anime Quotes API - Uninstall Script
# Supports: Linux, macOS, BSD
# Repository: github.com/apimgr/anime
################################################################################

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging functions
log() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $*"
}

# Detect operating system
detect_os() {
    local os_name
    os_name=$(uname -s)

    case "$os_name" in
        Linux)
            echo "linux"
            ;;
        Darwin)
            echo "macos"
            ;;
        FreeBSD|OpenBSD|NetBSD)
            echo "bsd"
            ;;
        *)
            log_error "Unsupported operating system: $os_name"
            exit 1
            ;;
    esac
}

# Detect init system (Linux only)
detect_init_system() {
    if [ -d /run/systemd/system ]; then
        echo "systemd"
    elif [ -f /sbin/openrc-run ]; then
        echo "openrc"
    elif [ -d /etc/runit ]; then
        echo "runit"
    elif [ -d /etc/s6 ]; then
        echo "s6"
    elif [ -f /sbin/init ]; then
        echo "sysvinit"
    else
        echo "unknown"
    fi
}

# Set directories based on OS
set_directories() {
    local os=$1

    case "$os" in
        linux)
            INSTALL_DIR="/usr/local/bin"
            CONFIG_DIR="/etc/anime"
            DATA_DIR="/var/lib/anime"
            LOG_DIR="/var/log/anime"
            SERVICE_USER="anime"
            SERVICE_GROUP="anime"
            ;;
        macos)
            INSTALL_DIR="/usr/local/bin"
            CONFIG_DIR="/Library/Application Support/Anime"
            DATA_DIR="/Library/Application Support/Anime/data"
            LOG_DIR="/Library/Logs/Anime"
            PLIST_PATH="/Library/LaunchDaemons/com.apimgr.anime.plist"
            ;;
        bsd)
            INSTALL_DIR="/usr/local/bin"
            CONFIG_DIR="/usr/local/etc/anime"
            DATA_DIR="/var/db/anime"
            LOG_DIR="/var/log/anime"
            SERVICE_USER="anime"
            SERVICE_GROUP="anime"
            ;;
    esac
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Prompt for confirmation
confirm_uninstall() {
    echo ""
    echo "=========================================="
    echo "ANIME API UNINSTALL"
    echo "=========================================="
    echo ""
    echo "This will remove:"
    echo "  - Binary: ${INSTALL_DIR}/anime"
    echo "  - Config: ${CONFIG_DIR}"
    echo "  - Data:   ${DATA_DIR}"
    echo "  - Logs:   ${LOG_DIR}"
    echo "  - Service files"
    echo "  - System user: ${SERVICE_USER:-N/A}"
    echo ""

    read -p "Do you want to create a backup before uninstalling? (Y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]] || [[ -z $REPLY ]]; then
        CREATE_BACKUP=true
    else
        CREATE_BACKUP=false
    fi

    echo ""
    log_warn "This action cannot be undone (unless you created a backup)!"
    read -p "Continue with uninstall? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log "Uninstall cancelled by user"
        exit 0
    fi
}

# Create backup before uninstall
create_backup() {
    if [ "$CREATE_BACKUP" != true ]; then
        return
    fi

    log_step "Creating backup before uninstall..."

    if [ -f "${INSTALL_DIR}/anime" ]; then
        # Check if backup script exists
        local backup_script="/usr/local/share/anime/backup.sh"
        if [ ! -f "$backup_script" ]; then
            backup_script="$(dirname "$0")/backup.sh"
        fi

        if [ -f "$backup_script" ]; then
            bash "$backup_script" --backup-dir "/var/backups/anime" || {
                log_warn "Backup failed, but continuing with uninstall"
            }
        else
            log_warn "Backup script not found, skipping backup"
        fi
    fi
}

# Stop service (Linux systemd)
stop_systemd_service() {
    log_step "Stopping systemd service..."

    if systemctl is-active --quiet anime.service; then
        systemctl stop anime.service
        log "Service stopped"
    fi

    if systemctl is-enabled --quiet anime.service 2>/dev/null; then
        systemctl disable anime.service
        log "Service disabled"
    fi
}

# Stop service (Linux OpenRC)
stop_openrc_service() {
    log_step "Stopping OpenRC service..."

    if rc-service anime status >/dev/null 2>&1; then
        rc-service anime stop
        log "Service stopped"
    fi

    rc-update del anime default 2>/dev/null || true
    log "Service removed from default runlevel"
}

# Stop service (Linux runit)
stop_runit_service() {
    log_step "Stopping runit service..."

    if [ -L /var/service/anime ]; then
        sv down anime
        rm -f /var/service/anime
        log "Service stopped and unlinked"
    fi
}

# Stop service (Linux s6)
stop_s6_service() {
    log_step "Stopping s6 service..."

    if [ -d /etc/s6/sv/anime ]; then
        s6-rc -d change anime 2>/dev/null || true
        s6-rc-bundle-update remove default anime 2>/dev/null || true
        log "Service stopped"
    fi
}

# Stop service (Linux sysvinit)
stop_sysvinit_service() {
    log_step "Stopping sysvinit service..."

    if [ -f /etc/init.d/anime ]; then
        /etc/init.d/anime stop || true

        if command -v update-rc.d >/dev/null 2>&1; then
            update-rc.d anime remove || true
        elif command -v chkconfig >/dev/null 2>&1; then
            chkconfig anime off || true
            chkconfig --del anime || true
        fi

        log "Service stopped"
    fi
}

# Stop service (macOS launchd)
stop_launchd_service() {
    log_step "Stopping launchd service..."

    if [ -f "$PLIST_PATH" ]; then
        if launchctl list | grep -q "com.apimgr.anime"; then
            launchctl unload "$PLIST_PATH"
            log "Service unloaded"
        fi
    fi
}

# Stop service (BSD rc.d)
stop_bsd_service() {
    log_step "Stopping BSD service..."

    if [ -f /usr/local/etc/rc.d/anime ]; then
        /usr/local/etc/rc.d/anime stop || true
        log "Service stopped"
    fi
}

# Remove service files (Linux)
remove_linux_service_files() {
    local init_system=$1

    log_step "Removing service files..."

    case "$init_system" in
        systemd)
            if [ -f /etc/systemd/system/anime.service ]; then
                rm -f /etc/systemd/system/anime.service
                systemctl daemon-reload
                log "Removed systemd service file"
            fi
            ;;
        openrc)
            if [ -f /etc/init.d/anime ]; then
                rm -f /etc/init.d/anime
                log "Removed OpenRC service file"
            fi
            ;;
        runit)
            if [ -d /etc/sv/anime ]; then
                rm -rf /etc/sv/anime
                log "Removed runit service directory"
            fi
            ;;
        s6)
            if [ -d /etc/s6/sv/anime ]; then
                rm -rf /etc/s6/sv/anime
                log "Removed s6 service directory"
            fi
            ;;
        sysvinit)
            if [ -f /etc/init.d/anime ]; then
                rm -f /etc/init.d/anime
                log "Removed sysvinit service file"
            fi
            ;;
    esac
}

# Remove service files (macOS)
remove_macos_service_files() {
    log_step "Removing launchd plist..."

    if [ -f "$PLIST_PATH" ]; then
        rm -f "$PLIST_PATH"
        log "Removed launchd plist"
    fi
}

# Remove service files (BSD)
remove_bsd_service_files() {
    log_step "Removing rc.d script..."

    if [ -f /usr/local/etc/rc.d/anime ]; then
        rm -f /usr/local/etc/rc.d/anime
        log "Removed rc.d script"
    fi

    # Remove from rc.conf
    if grep -q "anime_enable=" /etc/rc.conf 2>/dev/null; then
        sed -i.bak '/anime_enable=/d' /etc/rc.conf
        log "Removed from rc.conf"
    fi
}

# Remove binary
remove_binary() {
    log_step "Removing binary..."

    if [ -f "${INSTALL_DIR}/anime" ]; then
        rm -f "${INSTALL_DIR}/anime"
        log "Removed binary: ${INSTALL_DIR}/anime"
    else
        log_warn "Binary not found: ${INSTALL_DIR}/anime"
    fi
}

# Remove directories
remove_directories() {
    log_step "Removing directories..."

    for dir in "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"; do
        if [ -d "$dir" ]; then
            rm -rf "$dir"
            log "Removed: $dir"
        fi
    done
}

# Remove system user
remove_user() {
    local os=$1

    if [ -z "$SERVICE_USER" ]; then
        return
    fi

    log_step "Removing system user..."

    if id "$SERVICE_USER" >/dev/null 2>&1; then
        case "$os" in
            linux)
                userdel "$SERVICE_USER" 2>/dev/null || true
                groupdel "$SERVICE_GROUP" 2>/dev/null || true
                ;;
            bsd)
                if command -v pw >/dev/null 2>&1; then
                    pw userdel "$SERVICE_USER" 2>/dev/null || true
                    pw groupdel "$SERVICE_GROUP" 2>/dev/null || true
                else
                    userdel "$SERVICE_USER" 2>/dev/null || true
                    groupdel "$SERVICE_GROUP" 2>/dev/null || true
                fi
                ;;
        esac
        log "Removed user: $SERVICE_USER"
    fi
}

# Remove additional files
remove_additional_files() {
    log_step "Removing additional files..."

    # Remove backup scripts
    [ -f /usr/local/share/anime/backup.sh ] && rm -f /usr/local/share/anime/backup.sh
    [ -d /usr/local/share/anime ] && rmdir /usr/local/share/anime 2>/dev/null || true

    # Remove NSSM (Windows compatibility on Linux)
    [ -f "${INSTALL_DIR}/nssm.exe" ] && rm -f "${INSTALL_DIR}/nssm.exe"

    log "Additional files cleaned up"
}

# Display completion message
display_completion() {
    echo ""
    echo "=========================================="
    echo "UNINSTALL COMPLETED"
    echo "=========================================="
    echo ""
    log "Anime API has been completely removed from your system"
    echo ""

    if [ "$CREATE_BACKUP" = true ]; then
        echo "Backup Location: /var/backups/anime/"
        echo "You can restore from backup if needed"
        echo ""
    fi

    echo "Thank you for using Anime API!"
    echo "=========================================="
    echo ""
}

# Main uninstall flow
main() {
    echo ""
    log "Starting Anime API uninstall..."
    echo ""

    # Pre-flight checks
    check_root

    # Detect system
    OS=$(detect_os)
    set_directories "$OS"

    log "Operating System: $OS"
    log "Binary: ${INSTALL_DIR}/anime"
    echo ""

    # Confirm uninstall
    confirm_uninstall

    # Create backup if requested
    create_backup

    # Stop service based on OS and init system
    case "$OS" in
        linux)
            INIT_SYSTEM=$(detect_init_system)
            log "Init System: $INIT_SYSTEM"

            case "$INIT_SYSTEM" in
                systemd)
                    stop_systemd_service
                    ;;
                openrc)
                    stop_openrc_service
                    ;;
                runit)
                    stop_runit_service
                    ;;
                s6)
                    stop_s6_service
                    ;;
                sysvinit)
                    stop_sysvinit_service
                    ;;
                *)
                    log_warn "Unknown init system, skipping service stop"
                    ;;
            esac

            remove_linux_service_files "$INIT_SYSTEM"
            ;;
        macos)
            stop_launchd_service
            remove_macos_service_files
            ;;
        bsd)
            stop_bsd_service
            remove_bsd_service_files
            ;;
    esac

    # Remove components
    remove_binary
    remove_directories
    remove_user "$OS"
    remove_additional_files

    # Display completion
    display_completion
}

# Run main function
main "$@"
