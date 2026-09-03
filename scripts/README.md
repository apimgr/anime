# Anime Quotes API - Installation Scripts

Comprehensive installation and management scripts for all supported platforms.

## Overview

This directory contains production-ready installation scripts that automate the deployment of the Anime Quotes API across different operating systems and architectures.

## Scripts

### Installation Scripts

#### `install-linux.sh`
**Comprehensive Linux installation script**

**Supported Distributions:**
- Ubuntu / Debian
- Fedora / RHEL / CentOS / Rocky / Alma
- Arch Linux
- Alpine Linux
- OpenSUSE
- And more...

**Supported Init Systems:**
- systemd (Ubuntu, Fedora, Arch, etc.)
- OpenRC (Alpine, Gentoo)
- runit (Void Linux, Artix)
- s6 (Alpine alternative)
- sysvinit (Legacy systems)

**Features:**
- Auto-detects distribution and init system
- Downloads correct binary for architecture (amd64/arm64)
- Creates system user and directories
- Installs and configures service
- Generates secure admin credentials
- Automatic service startup

**Usage:**
```bash
# Download and run
curl -fsSL https://github.com/apimgr/anime/raw/main/scripts/install-linux.sh | sudo bash

# Or download first
wget https://github.com/apimgr/anime/raw/main/scripts/install-linux.sh
chmod +x install-linux.sh
sudo ./install-linux.sh

# Specify version
VERSION=0.0.2 sudo ./install-linux.sh
```

**Service Management:**
```bash
# systemd
sudo systemctl status anime
sudo systemctl restart anime
sudo journalctl -u anime -f

# OpenRC
sudo rc-service anime status
sudo rc-service anime restart
tail -f /var/log/anime/anime.log

# runit
sudo sv status anime
sudo sv restart anime
```

---

#### `install-macos.sh`
**macOS installation script with launchd integration**

**Supported Versions:**
- macOS 10.15 (Catalina) and later
- Both Intel (amd64) and Apple Silicon (arm64)

**Features:**
- Auto-detects architecture (Intel vs Apple Silicon)
- Downloads appropriate binary
- Creates launchd plist for automatic startup
- Removes quarantine attributes
- Configures system directories

**Usage:**
```bash
# Download and run
curl -fsSL https://github.com/apimgr/anime/raw/main/scripts/install-macos.sh | sudo bash

# Or download first
curl -O https://github.com/apimgr/anime/raw/main/scripts/install-macos.sh
chmod +x install-macos.sh
sudo ./install-macos.sh
```

**Service Management:**
```bash
# Check status
sudo launchctl list | grep anime

# Start service
sudo launchctl load /Library/LaunchDaemons/com.apimgr.anime.plist

# Stop service
sudo launchctl unload /Library/LaunchDaemons/com.apimgr.anime.plist

# View logs
tail -f /Library/Logs/Anime/anime.log
```

**Directories:**
- Config: `/Library/Application Support/Anime/`
- Data: `/Library/Application Support/Anime/data/`
- Logs: `/Library/Logs/Anime/`

---

#### `install-windows.ps1`
**Windows PowerShell installation script with NSSM**

**Supported Versions:**
- Windows 10 / 11
- Windows Server 2016 and later
- Both amd64 and arm64

**Features:**
- Downloads and installs NSSM (Non-Sucking Service Manager)
- Creates Windows service
- Configures firewall rules
- Auto-detects architecture
- Log rotation support

**Usage:**
```powershell
# Run as Administrator
# Download and run
Invoke-WebRequest -Uri "https://github.com/apimgr/anime/raw/main/scripts/install-windows.ps1" -OutFile "install-windows.ps1"
powershell -ExecutionPolicy Bypass -File .\install-windows.ps1

# Or one-liner (requires unrestricted execution policy)
iex ((New-Object System.Net.WebClient).DownloadString('https://github.com/apimgr/anime/raw/main/scripts/install-windows.ps1'))
```

**Service Management:**
```powershell
# Check status
Get-Service -Name AnimeAPI

# Start service
Start-Service -Name AnimeAPI

# Stop service
Stop-Service -Name AnimeAPI

# Restart service
Restart-Service -Name AnimeAPI

# View logs
Get-Content C:\ProgramData\Anime\logs\anime.log -Tail 50 -Wait
```

**Directories:**
- Install: `C:\Program Files\Anime\`
- Config: `C:\ProgramData\Anime\config\`
- Data: `C:\ProgramData\Anime\data\`
- Logs: `C:\ProgramData\Anime\logs\`

---

#### `install-bsd.sh`
**BSD installation script (FreeBSD, OpenBSD, NetBSD)**

**Supported Systems:**
- FreeBSD 12.0+
- OpenBSD 6.8+
- NetBSD 9.0+

**Features:**
- Auto-detects BSD variant
- Creates rc.d service script
- Configures rc.conf
- Supports both amd64 and arm64

**Usage:**
```bash
# FreeBSD
fetch https://github.com/apimgr/anime/raw/main/scripts/install-bsd.sh
chmod +x install-bsd.sh
sudo ./install-bsd.sh

# OpenBSD
ftp https://github.com/apimgr/anime/raw/main/scripts/install-bsd.sh
chmod +x install-bsd.sh
doas sh install-bsd.sh
```

**Service Management:**
```bash
# FreeBSD
sudo service anime status
sudo service anime restart

# OpenBSD/NetBSD
sudo /usr/local/etc/rc.d/anime status
sudo /usr/local/etc/rc.d/anime restart
```

**Directories:**
- Config: `/usr/local/etc/anime/`
- Data: `/var/db/anime/`
- Logs: `/var/log/anime/`

---

### Management Scripts

#### `backup.sh`
**Database and configuration backup script**

**Features:**
- Auto-detects installation directories
- Creates compressed archives
- Generates restore script automatically
- Creates SQL dumps for safety
- Retention policy support (default: 30 days)
- Checksums for backup verification
- Cross-platform (Linux, macOS, BSD)

**Usage:**
```bash
# Basic backup (auto-detect directories)
sudo ./backup.sh

# Custom backup directory
sudo ./backup.sh --backup-dir /path/to/backups

# Set retention period (days)
sudo ./backup.sh --retention 60

# Override directories
sudo ./backup.sh \
  --config-dir /etc/anime \
  --data-dir /var/lib/anime \
  --backup-dir /backups/anime
```

**Backup Contents:**
- Configuration files (`/etc/anime/`)
- SQLite database (with proper locking)
- SQL dump (gzipped)
- Data directory
- Metadata file
- Restore script

**Backup Location:**
- Default: `/var/backups/anime/`
- Format: `anime-backup-YYYYMMDD-HHMMSS.tar.gz`
- Checksum: `.tar.gz.sha256`

**Restore from Backup:**
```bash
# Extract backup
tar -xzf anime-backup-20231016-120000.tar.gz
cd anime-backup-20231016-120000

# Run restore script
sudo ./restore.sh
```

**Automated Backups (cron):**
```bash
# Daily backup at 2 AM
0 2 * * * /usr/local/share/anime/backup.sh --backup-dir /var/backups/anime >> /var/log/anime/backup.log 2>&1

# Weekly backup with 90-day retention
0 3 * * 0 /usr/local/share/anime/backup.sh --retention 90 --backup-dir /backups/anime
```

---

#### `uninstall.sh`
**Complete removal script for all platforms**

**Features:**
- Cross-platform (Linux, macOS, BSD)
- Auto-detects OS and init system
- Optional backup before uninstall
- Stops and removes service
- Removes all files and directories
- Removes system user
- Safe confirmation prompts

**Usage:**
```bash
# Run uninstall
sudo ./uninstall.sh

# The script will prompt for:
# 1. Whether to create backup (recommended)
# 2. Final confirmation before removal
```

**What Gets Removed:**
- Binary (`/usr/local/bin/anime`)
- Configuration directory
- Data directory (including database)
- Log directory
- Service files (systemd/launchd/rc.d)
- System user and group
- Additional files (NSSM on Windows, etc.)

**Backup Before Uninstall:**
The script will ask if you want to create a backup. If yes:
- Creates full backup in `/var/backups/anime/`
- Backup can be used to restore later
- Includes all data, config, and database

---

## Quick Start Guide

### Linux
```bash
# Install
curl -fsSL https://github.com/apimgr/anime/raw/main/scripts/install-linux.sh | sudo bash

# Access API
curl http://localhost:8080/api/v1/random

# View credentials
sudo cat /etc/anime/admin_credentials

# Check service
sudo systemctl status anime
```

### macOS
```bash
# Install
curl -fsSL https://github.com/apimgr/anime/raw/main/scripts/install-macos.sh | sudo bash

# Access API
curl http://localhost:8080/api/v1/random

# View credentials
sudo cat "/Library/Application Support/Anime/admin_credentials"

# Check service
sudo launchctl list | grep anime
```

### Windows (PowerShell as Administrator)
```powershell
# Install
Invoke-WebRequest -Uri "https://github.com/apimgr/anime/raw/main/scripts/install-windows.ps1" -OutFile "install-windows.ps1"
.\install-windows.ps1

# Access API
Invoke-RestMethod -Uri http://localhost:8080/api/v1/random

# View credentials
Get-Content "C:\ProgramData\Anime\config\admin_credentials"

# Check service
Get-Service -Name AnimeAPI
```

### FreeBSD
```bash
# Install
fetch https://github.com/apimgr/anime/raw/main/scripts/install-bsd.sh
chmod +x install-bsd.sh
sudo ./install-bsd.sh

# Access API
curl http://localhost:8080/api/v1/random

# View credentials
sudo cat /usr/local/etc/anime/admin_credentials

# Check service
sudo service anime status
```

---

## Environment Variables

All installation scripts support environment variables:

```bash
# Version to install
VERSION=0.0.2

# Installation directories (advanced)
INSTALL_DIR=/opt/anime/bin
CONFIG_DIR=/opt/anime/etc
DATA_DIR=/opt/anime/data
LOG_DIR=/opt/anime/logs

# Service configuration
SERVICE_USER=anime
SERVICE_GROUP=anime
```

---

## Default Credentials

After installation, default admin credentials are generated and saved to:

- **Linux**: `/etc/anime/admin_credentials`
- **macOS**: `/Library/Application Support/Anime/admin_credentials`
- **Windows**: `C:\ProgramData\Anime\config\admin_credentials`
- **BSD**: `/usr/local/etc/anime/admin_credentials`

**IMPORTANT:** Change these credentials immediately after first login!

**Default format:**
```
ADMIN_USER=administrator
ADMIN_PASSWORD=<randomly-generated-16-chars>
ADMIN_TOKEN=<randomly-generated-64-hex>
```

---

## Directory Structure

### Linux (System Install)
```
/usr/local/bin/anime          # Binary
/etc/anime/                   # Configuration
/var/lib/anime/               # Data
/var/lib/anime/db/            # Database
/var/log/anime/               # Logs
```

### macOS (System Install)
```
/usr/local/bin/anime                              # Binary
/Library/Application Support/Anime/               # Configuration
/Library/Application Support/Anime/data/          # Data
/Library/Application Support/Anime/data/db/       # Database
/Library/Logs/Anime/                              # Logs
```

### Windows (System Install)
```
C:\Program Files\Anime\anime.exe                  # Binary
C:\ProgramData\Anime\config\                      # Configuration
C:\ProgramData\Anime\data\                        # Data
C:\ProgramData\Anime\data\db\                     # Database
C:\ProgramData\Anime\logs\                        # Logs
```

### BSD (System Install)
```
/usr/local/bin/anime          # Binary
/usr/local/etc/anime/         # Configuration
/var/db/anime/                # Data
/var/db/anime/db/             # Database
/var/log/anime/               # Logs
```

---

## Supported Architectures

All installation scripts support:
- **amd64** (x86_64) - Intel/AMD 64-bit
- **arm64** (aarch64) - ARM 64-bit (Raspberry Pi 4+, Apple Silicon, AWS Graviton, etc.)

Architecture is detected automatically.

---

## Security Considerations

### Permissions
- Binary: `755` (root:root)
- Config directory: `750` (anime:anime)
- Data directory: `750` (anime:anime)
- Database directory: `770` (anime:anime)
- Log directory: `750` (anime:anime)
- Credentials file: `600` (anime:anime)

### Service User
- Non-privileged user: `anime`
- No shell access: `/bin/false` or `/sbin/nologin`
- No home directory
- Member of `anime` group only

### Network Binding
- Default: `::` (dual-stack IPv4+IPv6)
- Default port: `8080`
- Change in service configuration if needed

### Firewall
- Linux: Manual configuration required (ufw/firewalld)
- macOS: Manual configuration via System Preferences
- Windows: Automatic rule creation (port 8080)
- BSD: Manual configuration (pf/ipfw)

---

## Troubleshooting

### Installation Failed
```bash
# Check installation log
cat /tmp/anime-install-*.log

# Verify binary download
curl -I https://github.com/apimgr/anime/releases/download/0.0.1/anime-linux-amd64

# Check system requirements
uname -m  # Should be x86_64 or aarch64
```

### Service Won't Start
```bash
# Check logs
sudo journalctl -u anime -n 50  # systemd
tail -50 /var/log/anime/error.log  # other systems

# Test binary manually
sudo -u anime /usr/local/bin/anime --version
sudo -u anime /usr/local/bin/anime --port 8081 --address 127.0.0.1

# Check permissions
ls -la /var/lib/anime/db/
ls -la /etc/anime/
```

### Permission Denied Errors
```bash
# Fix ownership
sudo chown -R anime:anime /etc/anime /var/lib/anime /var/log/anime

# Fix permissions
sudo chmod 750 /etc/anime /var/lib/anime /var/log/anime
sudo chmod 770 /var/lib/anime/db
sudo chmod 600 /etc/anime/admin_credentials
```

### Database Locked
```bash
# Check for running processes
ps aux | grep anime

# Stop service properly
sudo systemctl stop anime  # systemd
sudo service anime stop    # other systems

# Check for stale locks
sudo fuser /var/lib/anime/db/anime.db
```

---

## Testing Installation

After installation, test the API:

```bash
# Health check
curl http://localhost:8080/api/v1/health

# Get random quote
curl http://localhost:8080/api/v1/random

# Get all quotes (paginated)
curl http://localhost:8080/api/v1/quotes

# Access admin panel (browser)
# http://localhost:8080/admin
# Use credentials from admin_credentials file
```

---

## Upgrading

To upgrade to a new version:

```bash
# 1. Create backup
sudo ./backup.sh

# 2. Stop service
sudo systemctl stop anime  # or appropriate command for your system

# 3. Download new binary
VERSION=0.0.2 wget https://github.com/apimgr/anime/releases/download/0.0.2/anime-linux-amd64

# 4. Replace binary
sudo mv anime-linux-amd64 /usr/local/bin/anime
sudo chmod 755 /usr/local/bin/anime

# 5. Start service
sudo systemctl start anime

# 6. Verify
curl http://localhost:8080/api/v1/health
```

Or simply re-run the installation script (it will backup existing binary):

```bash
VERSION=0.0.2 sudo ./install-linux.sh
```

---

## Support

- **Documentation**: https://github.com/apimgr/anime
- **Issues**: https://github.com/apimgr/anime/issues
- **Releases**: https://github.com/apimgr/anime/releases

---

## License

MIT License - See [LICENSE.md](../LICENSE.md) for details
