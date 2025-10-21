#!/bin/bash
################################################################################
# Anime Quotes API - Backup Script
# Backs up database and configuration files
# Repository: github.com/apimgr/anime
################################################################################

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Default configuration
BACKUP_DIR="${BACKUP_DIR:-/var/backups/anime}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_NAME="anime-backup-${TIMESTAMP}"

# Auto-detect directories based on OS
detect_directories() {
    if [ -d "/etc/anime" ]; then
        # Linux system install
        CONFIG_DIR="${CONFIG_DIR:-/etc/anime}"
        DATA_DIR="${DATA_DIR:-/var/lib/anime}"
        LOG_DIR="${LOG_DIR:-/var/log/anime}"
    elif [ -d "/Library/Application Support/Anime" ]; then
        # macOS system install
        CONFIG_DIR="${CONFIG_DIR:-/Library/Application Support/Anime}"
        DATA_DIR="${DATA_DIR:-/Library/Application Support/Anime/data}"
        LOG_DIR="${LOG_DIR:-/Library/Logs/Anime}"
    elif [ -d "/usr/local/etc/anime" ]; then
        # BSD system install
        CONFIG_DIR="${CONFIG_DIR:-/usr/local/etc/anime}"
        DATA_DIR="${DATA_DIR:-/var/db/anime}"
        LOG_DIR="${LOG_DIR:-/var/log/anime}"
    elif [ -d "$HOME/.config/anime" ]; then
        # User install (Linux)
        CONFIG_DIR="${CONFIG_DIR:-$HOME/.config/anime}"
        DATA_DIR="${DATA_DIR:-$HOME/.local/share/anime}"
        LOG_DIR="${LOG_DIR:-$HOME/.local/state/anime}"
    else
        echo -e "${RED}[ERROR]${NC} Could not detect Anime API installation"
        exit 1
    fi
}

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

# Check if running as appropriate user
check_permissions() {
    if [ ! -r "$CONFIG_DIR" ] || [ ! -r "$DATA_DIR" ]; then
        log_error "Insufficient permissions to read data directories"
        log_error "Run as root or the anime service user"
        exit 1
    fi
}

# Create backup directory
create_backup_dir() {
    log_step "Creating backup directory..."

    if [ ! -d "$BACKUP_DIR" ]; then
        mkdir -p "$BACKUP_DIR"
        log "Created backup directory: $BACKUP_DIR"
    fi

    # Create temporary directory for this backup
    TEMP_BACKUP_DIR="${BACKUP_DIR}/${BACKUP_NAME}"
    mkdir -p "$TEMP_BACKUP_DIR"
}

# Backup configuration files
backup_config() {
    log_step "Backing up configuration files..."

    if [ -d "$CONFIG_DIR" ]; then
        mkdir -p "${TEMP_BACKUP_DIR}/config"
        cp -r "$CONFIG_DIR"/* "${TEMP_BACKUP_DIR}/config/" 2>/dev/null || true
        log "Configuration backed up"
    else
        log_warn "Configuration directory not found: $CONFIG_DIR"
    fi
}

# Backup database
backup_database() {
    log_step "Backing up database..."

    local db_file="${DATA_DIR}/db/anime.db"

    if [ -f "$db_file" ]; then
        mkdir -p "${TEMP_BACKUP_DIR}/db"

        # Check if SQLite is available for proper backup
        if command -v sqlite3 >/dev/null 2>&1; then
            # Use SQLite backup command for consistent backup
            sqlite3 "$db_file" ".backup '${TEMP_BACKUP_DIR}/db/anime.db'"
            log "Database backed up using SQLite backup command"
        else
            # Fallback to file copy (less safe but works)
            cp "$db_file" "${TEMP_BACKUP_DIR}/db/anime.db"
            log_warn "Database backed up using file copy (SQLite not found)"
        fi

        # Create SQL dump for extra safety
        if command -v sqlite3 >/dev/null 2>&1; then
            sqlite3 "$db_file" ".dump" | gzip > "${TEMP_BACKUP_DIR}/db/anime.sql.gz"
            log "SQL dump created"
        fi
    else
        log_warn "Database file not found: $db_file"
    fi
}

# Backup data directory
backup_data() {
    log_step "Backing up data directory..."

    if [ -d "$DATA_DIR" ]; then
        mkdir -p "${TEMP_BACKUP_DIR}/data"

        # Exclude database (already backed up) and large files
        rsync -a --exclude='db/' --exclude='*.log' \
            "$DATA_DIR/" "${TEMP_BACKUP_DIR}/data/" 2>/dev/null || \
        cp -r "$DATA_DIR"/* "${TEMP_BACKUP_DIR}/data/" 2>/dev/null || true

        log "Data directory backed up"
    else
        log_warn "Data directory not found: $DATA_DIR"
    fi
}

# Create metadata file
create_metadata() {
    log_step "Creating backup metadata..."

    cat > "${TEMP_BACKUP_DIR}/backup-info.txt" <<EOF
Anime Quotes API Backup
=======================

Backup Date:     $(date)
Backup Name:     ${BACKUP_NAME}
Hostname:        $(hostname)
Operating System: $(uname -s) $(uname -r)

Directories Backed Up:
- Config: ${CONFIG_DIR}
- Data:   ${DATA_DIR}

Backup Contents:
$(du -sh "${TEMP_BACKUP_DIR}"/* 2>/dev/null || echo "N/A")

Total Backup Size: $(du -sh "${TEMP_BACKUP_DIR}" | awk '{print $1}')

Restore Instructions:
---------------------
1. Stop the Anime API service
2. Extract backup: tar -xzf ${BACKUP_NAME}.tar.gz
3. Restore files to their original locations
4. Fix permissions: chown -R anime:anime [directories]
5. Start the service

For automated restore, use: ./restore.sh ${BACKUP_NAME}.tar.gz
EOF

    log "Metadata created"
}

# Create restore script
create_restore_script() {
    cat > "${TEMP_BACKUP_DIR}/restore.sh" <<'RESTORE_EOF'
#!/bin/bash
################################################################################
# Anime Quotes API - Restore Script
# Auto-generated with backup
################################################################################

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# Check root
if [ "$EUID" -ne 0 ]; then
    log_error "This script must be run as root"
    exit 1
fi

# Confirmation
echo "This will restore the Anime API from backup."
read -p "Continue? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log "Restore cancelled"
    exit 0
fi

# Detect service manager
detect_service() {
    if command -v systemctl >/dev/null 2>&1; then
        echo "systemd"
    elif command -v launchctl >/dev/null 2>&1; then
        echo "launchd"
    elif command -v service >/dev/null 2>&1; then
        echo "service"
    else
        echo "unknown"
    fi
}

# Stop service
log "Stopping Anime API service..."
SERVICE_MGR=$(detect_service)
case "$SERVICE_MGR" in
    systemd)
        systemctl stop anime || true
        ;;
    launchd)
        launchctl unload /Library/LaunchDaemons/com.apimgr.anime.plist || true
        ;;
    service)
        service anime stop || /usr/local/etc/rc.d/anime stop || true
        ;;
    *)
        log_warn "Could not stop service automatically"
        ;;
esac

# Restore files
log "Restoring configuration..."
[ -d config ] && cp -r config/* "CONFIG_DIR_PLACEHOLDER/" 2>/dev/null || true

log "Restoring database..."
[ -d db ] && cp -r db/* "DATA_DIR_PLACEHOLDER/db/" 2>/dev/null || true

log "Restoring data..."
[ -d data ] && cp -r data/* "DATA_DIR_PLACEHOLDER/" 2>/dev/null || true

# Fix permissions
log "Fixing permissions..."
if id anime >/dev/null 2>&1; then
    chown -R anime:anime "CONFIG_DIR_PLACEHOLDER" "DATA_DIR_PLACEHOLDER" 2>/dev/null || true
fi

# Start service
log "Starting Anime API service..."
case "$SERVICE_MGR" in
    systemd)
        systemctl start anime
        ;;
    launchd)
        launchctl load /Library/LaunchDaemons/com.apimgr.anime.plist
        ;;
    service)
        service anime start || /usr/local/etc/rc.d/anime start
        ;;
esac

log "Restore completed successfully!"
RESTORE_EOF

    # Replace placeholders
    sed -i.bak "s|CONFIG_DIR_PLACEHOLDER|${CONFIG_DIR}|g" "${TEMP_BACKUP_DIR}/restore.sh"
    sed -i.bak "s|DATA_DIR_PLACEHOLDER|${DATA_DIR}|g" "${TEMP_BACKUP_DIR}/restore.sh"
    rm -f "${TEMP_BACKUP_DIR}/restore.sh.bak"

    chmod +x "${TEMP_BACKUP_DIR}/restore.sh"
    log "Restore script created"
}

# Compress backup
compress_backup() {
    log_step "Compressing backup..."

    cd "$BACKUP_DIR"
    tar -czf "${BACKUP_NAME}.tar.gz" "${BACKUP_NAME}"

    if [ $? -eq 0 ]; then
        # Calculate size
        local backup_size
        backup_size=$(du -h "${BACKUP_NAME}.tar.gz" | awk '{print $1}')

        log "Backup compressed: ${BACKUP_NAME}.tar.gz (${backup_size})"

        # Calculate checksum
        if command -v sha256sum >/dev/null 2>&1; then
            sha256sum "${BACKUP_NAME}.tar.gz" > "${BACKUP_NAME}.tar.gz.sha256"
            log "Checksum created: ${BACKUP_NAME}.tar.gz.sha256"
        elif command -v shasum >/dev/null 2>&1; then
            shasum -a 256 "${BACKUP_NAME}.tar.gz" > "${BACKUP_NAME}.tar.gz.sha256"
            log "Checksum created: ${BACKUP_NAME}.tar.gz.sha256"
        fi

        # Remove temporary directory
        rm -rf "${TEMP_BACKUP_DIR}"
    else
        log_error "Failed to compress backup"
        exit 1
    fi
}

# Clean old backups
cleanup_old_backups() {
    log_step "Cleaning up old backups (retention: ${RETENTION_DAYS} days)..."

    local deleted=0
    find "$BACKUP_DIR" -name "anime-backup-*.tar.gz" -type f -mtime "+${RETENTION_DAYS}" -print0 | while IFS= read -r -d '' file; do
        log "Deleting old backup: $(basename "$file")"
        rm -f "$file" "${file}.sha256"
        deleted=$((deleted + 1))
    done

    if [ "$deleted" -gt 0 ]; then
        log "Deleted $deleted old backup(s)"
    else
        log "No old backups to delete"
    fi
}

# Display backup information
display_info() {
    local backup_path="${BACKUP_DIR}/${BACKUP_NAME}.tar.gz"
    local backup_size
    backup_size=$(du -h "$backup_path" | awk '{print $1}')

    echo ""
    echo "=========================================="
    echo "BACKUP COMPLETED"
    echo "=========================================="
    echo "Backup File:  $backup_path"
    echo "Backup Size:  $backup_size"
    echo "Timestamp:    $TIMESTAMP"
    echo ""
    echo "Backed Up:"
    echo "  - Config:   $CONFIG_DIR"
    echo "  - Data:     $DATA_DIR"
    echo "  - Database: $DATA_DIR/db/anime.db"
    echo ""
    echo "Retention:    $RETENTION_DAYS days"
    echo ""
    echo "To restore this backup:"
    echo "  tar -xzf $backup_path"
    echo "  cd ${BACKUP_NAME}"
    echo "  sudo ./restore.sh"
    echo ""
    echo "=========================================="
}

# Main backup flow
main() {
    echo ""
    log "Starting Anime API backup..."
    echo ""

    # Detect directories
    detect_directories

    log "Configuration directory: $CONFIG_DIR"
    log "Data directory: $DATA_DIR"
    log "Backup directory: $BACKUP_DIR"
    echo ""

    # Check permissions
    check_permissions

    # Create backup
    create_backup_dir
    backup_config
    backup_database
    backup_data
    create_metadata
    create_restore_script
    compress_backup

    # Cleanup old backups
    if [ "$RETENTION_DAYS" -gt 0 ]; then
        cleanup_old_backups
    fi

    # Display information
    display_info

    log "Backup completed successfully!"
}

# Parse command line arguments
while [ $# -gt 0 ]; do
    case "$1" in
        --backup-dir)
            BACKUP_DIR="$2"
            shift 2
            ;;
        --retention)
            RETENTION_DAYS="$2"
            shift 2
            ;;
        --config-dir)
            CONFIG_DIR="$2"
            shift 2
            ;;
        --data-dir)
            DATA_DIR="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --backup-dir DIR     Backup directory (default: /var/backups/anime)"
            echo "  --retention DAYS     Retention period in days (default: 30)"
            echo "  --config-dir DIR     Override config directory"
            echo "  --data-dir DIR       Override data directory"
            echo "  --help               Show this help message"
            echo ""
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main function
main "$@"
