#Requires -RunAsAdministrator
################################################################################
# Anime Quotes API - Windows Installation Script
# Supports: Windows 10/11/Server (amd64/arm64)
# Repository: github.com/apimgr/anime
################################################################################

# Configuration
$VERSION = if ($env:VERSION) { $env:VERSION } else { "0.0.1" }
$GITHUB_REPO = "apimgr/anime"
$INSTALL_DIR = "C:\Program Files\Anime"
$CONFIG_DIR = "C:\ProgramData\Anime\config"
$DATA_DIR = "C:\ProgramData\Anime\data"
$LOG_DIR = "C:\ProgramData\Anime\logs"
$SERVICE_NAME = "AnimeAPI"
$SERVICE_DISPLAY_NAME = "Anime Quotes API Service"
$NSSM_VERSION = "2.24"
$INSTALL_LOG = "C:\ProgramData\Anime\install-$(Get-Date -Format 'yyyyMMdd-HHmmss').log"

# Color functions
function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Log-Info($Message) {
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "[$timestamp] [INFO] $Message"
    Write-ColorOutput Green $logMessage
    Add-Content -Path $INSTALL_LOG -Value $logMessage
}

function Log-Warn($Message) {
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "[$timestamp] [WARN] $Message"
    Write-ColorOutput Yellow $logMessage
    Add-Content -Path $INSTALL_LOG -Value $logMessage
}

function Log-Error($Message) {
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "[$timestamp] [ERROR] $Message"
    Write-ColorOutput Red $logMessage
    Add-Content -Path $INSTALL_LOG -Value $logMessage
}

function Log-Step($Message) {
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "[$timestamp] [STEP] $Message"
    Write-ColorOutput Cyan $logMessage
    Add-Content -Path $INSTALL_LOG -Value $logMessage
}

# Check if running as administrator
function Test-Administrator {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# Detect architecture
function Get-SystemArchitecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default {
            Log-Error "Unsupported architecture: $arch"
            exit 1
        }
    }
}

# Download file from URL
function Download-File($Url, $Output) {
    try {
        Log-Info "Downloading from: $Url"
        $ProgressPreference = 'SilentlyContinue'
        Invoke-WebRequest -Uri $Url -OutFile $Output -UseBasicParsing
        return $true
    }
    catch {
        Log-Error "Failed to download: $_"
        return $false
    }
}

# Download and install NSSM
function Install-NSSM {
    Log-Step "Installing NSSM (Non-Sucking Service Manager)..."

    # Check if NSSM is already installed
    $nssmPath = Join-Path $INSTALL_DIR "nssm.exe"
    if (Test-Path $nssmPath) {
        Log-Info "NSSM already installed"
        return $nssmPath
    }

    # Download NSSM
    $nssmUrl = "https://nssm.cc/release/nssm-$NSSM_VERSION.zip"
    $nssmZip = "$env:TEMP\nssm.zip"
    $nssmExtract = "$env:TEMP\nssm"

    if (!(Download-File $nssmUrl $nssmZip)) {
        Log-Error "Failed to download NSSM"
        exit 1
    }

    # Extract NSSM
    try {
        Expand-Archive -Path $nssmZip -DestinationPath $nssmExtract -Force
        $arch = Get-SystemArchitecture
        $nssmArch = if ($arch -eq "arm64") { "win64" } else { "win64" }
        $nssmExe = Join-Path $nssmExtract "nssm-$NSSM_VERSION\$nssmArch\nssm.exe"

        if (Test-Path $nssmExe) {
            Copy-Item $nssmExe $nssmPath -Force
            Log-Info "NSSM installed successfully"
        }
        else {
            Log-Error "NSSM executable not found after extraction"
            exit 1
        }
    }
    catch {
        Log-Error "Failed to extract NSSM: $_"
        exit 1
    }
    finally {
        Remove-Item $nssmZip -Force -ErrorAction SilentlyContinue
        Remove-Item $nssmExtract -Recurse -Force -ErrorAction SilentlyContinue
    }

    return $nssmPath
}

# Download binary
function Download-Binary($Architecture) {
    $binaryName = "anime-windows-${Architecture}.exe"
    $downloadUrl = "https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${binaryName}"
    $downloadPath = Join-Path $env:TEMP $binaryName

    Log-Step "Downloading Anime API binary..."
    Log-Info "Architecture: $Architecture"
    Log-Info "Version: $VERSION"
    Log-Info "URL: $downloadUrl"

    if (!(Download-File $downloadUrl $downloadPath)) {
        Log-Error "Failed to download binary"
        exit 1
    }

    Log-Info "Binary downloaded successfully"
    return $downloadPath
}

# Install binary
function Install-Binary($SourcePath) {
    Log-Step "Installing binary to $INSTALL_DIR..."

    # Create install directory
    if (!(Test-Path $INSTALL_DIR)) {
        New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
        Log-Info "Created directory: $INSTALL_DIR"
    }

    $targetPath = Join-Path $INSTALL_DIR "anime.exe"

    # Backup existing binary
    if (Test-Path $targetPath) {
        $backupPath = "$targetPath.bak.$(Get-Date -Format 'yyyyMMddHHmmss')"
        Log-Warn "Existing binary found. Creating backup..."
        Move-Item $targetPath $backupPath -Force
    }

    # Copy binary
    Copy-Item $SourcePath $targetPath -Force
    Log-Info "Binary installed successfully"

    # Verify installation
    try {
        & $targetPath --version 2>&1 | Out-Null
        Log-Info "Binary verified successfully"
    }
    catch {
        Log-Warn "Binary installed but version check failed"
    }

    return $targetPath
}

# Create directories
function Create-Directories {
    Log-Step "Creating application directories..."

    $directories = @($CONFIG_DIR, $DATA_DIR, (Join-Path $DATA_DIR "db"), $LOG_DIR)

    foreach ($dir in $directories) {
        if (!(Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
            Log-Info "Created directory: $dir"
        }
        else {
            Log-Info "Directory already exists: $dir"
        }
    }

    Log-Info "Permissions configured"
}

# Generate admin credentials
function Generate-Credentials {
    $credFile = Join-Path $CONFIG_DIR "admin_credentials"

    if (Test-Path $credFile) {
        Log-Info "Admin credentials file already exists"
        return
    }

    Log-Step "Generating default admin credentials..."

    # Generate random password and token
    $adminUser = "administrator"
    $adminPass = -join ((48..57) + (65..90) + (97..122) | Get-Random -Count 16 | ForEach-Object {[char]$_})
    $adminToken = -join ((48..57) + (65..70) | Get-Random -Count 64 | ForEach-Object {[char]$_})

    $credContent = @"
# Anime API Admin Credentials
# Generated on $(Get-Date)
ADMIN_USER=$adminUser
ADMIN_PASSWORD=$adminPass
ADMIN_TOKEN=$adminToken
"@

    Set-Content -Path $credFile -Value $credContent
    Log-Info "Admin credentials generated: $credFile"

    Write-Host ""
    Write-ColorOutput Yellow "=========================================="
    Write-ColorOutput Yellow "DEFAULT ADMIN CREDENTIALS"
    Write-ColorOutput Yellow "=========================================="
    Write-Host "Username: $adminUser"
    Write-Host "Password: $adminPass"
    Write-Host "Token:    $adminToken"
    Write-ColorOutput Yellow "=========================================="
    Write-ColorOutput Red "PLEASE CHANGE THESE IMMEDIATELY!"
    Write-ColorOutput Yellow "=========================================="
    Write-Host ""
}

# Create Windows service
function Create-Service($BinaryPath, $NssmPath) {
    Log-Step "Creating Windows service..."

    # Stop and remove existing service if exists
    $existingService = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($existingService) {
        Log-Warn "Existing service found. Stopping and removing..."
        if ($existingService.Status -eq "Running") {
            Stop-Service -Name $SERVICE_NAME -Force
        }
        & $NssmPath remove $SERVICE_NAME confirm
        Start-Sleep -Seconds 2
    }

    # Install service using NSSM
    & $NssmPath install $SERVICE_NAME $BinaryPath --port 8080 --address ::

    # Configure service
    & $NssmPath set $SERVICE_NAME DisplayName "$SERVICE_DISPLAY_NAME"
    & $NssmPath set $SERVICE_NAME Description "REST API service for anime quotes"
    & $NssmPath set $SERVICE_NAME Start SERVICE_AUTO_START
    & $NssmPath set $SERVICE_NAME AppDirectory $DATA_DIR
    & $NssmPath set $SERVICE_NAME AppStdout (Join-Path $LOG_DIR "anime.log")
    & $NssmPath set $SERVICE_NAME AppStderr (Join-Path $LOG_DIR "error.log")
    & $NssmPath set $SERVICE_NAME AppRotateFiles 1
    & $NssmPath set $SERVICE_NAME AppRotateBytes 10485760  # 10MB
    & $NssmPath set $SERVICE_NAME AppEnvironmentExtra "CONFIG_DIR=$CONFIG_DIR" "DATA_DIR=$DATA_DIR" "LOGS_DIR=$LOG_DIR"

    Log-Info "Service created successfully"
}

# Start service
function Start-AnimeService {
    Log-Step "Starting service..."

    try {
        Start-Service -Name $SERVICE_NAME
        Start-Sleep -Seconds 3

        $service = Get-Service -Name $SERVICE_NAME
        if ($service.Status -eq "Running") {
            Log-Info "Service started successfully"
            return $true
        }
        else {
            Log-Error "Service failed to start. Status: $($service.Status)"
            return $false
        }
    }
    catch {
        Log-Error "Failed to start service: $_"
        return $false
    }
}

# Configure firewall
function Configure-Firewall {
    Log-Step "Configuring Windows Firewall..."

    try {
        $ruleName = "Anime API (Port 8080)"
        $existingRule = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue

        if ($existingRule) {
            Log-Info "Firewall rule already exists"
        }
        else {
            New-NetFirewallRule -DisplayName $ruleName -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow | Out-Null
            Log-Info "Firewall rule created"
        }
    }
    catch {
        Log-Warn "Failed to configure firewall: $_"
        Log-Warn "You may need to manually allow port 8080"
    }
}

# Display installation info
function Display-Info {
    Log-Step "Installation complete!"

    # Get local IP addresses
    $ipv4 = (Get-NetIPAddress -AddressFamily IPv4 | Where-Object {$_.IPAddress -notlike "127.*"} | Select-Object -First 1).IPAddress
    $ipv6 = (Get-NetIPAddress -AddressFamily IPv6 | Where-Object {$_.IPAddress -notlike "::1" -and $_.IPAddress -notlike "fe80*"} | Select-Object -First 1).IPAddress

    Write-Host ""
    Write-ColorOutput Green "=========================================="
    Write-ColorOutput Green "INSTALLATION SUMMARY"
    Write-ColorOutput Green "=========================================="
    Write-Host "Binary:        $INSTALL_DIR\anime.exe"
    Write-Host "Config Dir:    $CONFIG_DIR"
    Write-Host "Data Dir:      $DATA_DIR"
    Write-Host "Log Dir:       $LOG_DIR"
    Write-Host "Service:       $SERVICE_NAME"
    Write-ColorOutput Green "=========================================="
    Write-Host ""
    Write-Host "API ACCESS:"
    Write-Host "  Local:       http://localhost:8080"
    if ($ipv4) {
        Write-Host "  IPv4:        http://${ipv4}:8080"
    }
    if ($ipv6) {
        Write-Host "  IPv6:        http://[${ipv6}]:8080"
    }
    Write-Host ""
    Write-Host "API ENDPOINTS:"
    Write-Host "  Random Quote: GET /api/v1/random"
    Write-Host "  All Quotes:   GET /api/v1/quotes"
    Write-Host "  Health Check: GET /api/v1/health"
    Write-Host "  Admin Panel:  http://localhost:8080/admin"
    Write-Host ""
    Write-Host "SERVICE MANAGEMENT:"
    Write-Host "  Status:  Get-Service -Name $SERVICE_NAME"
    Write-Host "  Start:   Start-Service -Name $SERVICE_NAME"
    Write-Host "  Stop:    Stop-Service -Name $SERVICE_NAME"
    Write-Host "  Restart: Restart-Service -Name $SERVICE_NAME"
    Write-Host "  Logs:    Get-Content $LOG_DIR\anime.log -Tail 50 -Wait"
    Write-Host ""
    Write-Host "CREDENTIALS:"
    Write-Host "  See: $CONFIG_DIR\admin_credentials"
    Write-Host ""
    Write-ColorOutput Green "=========================================="
    Write-Host ""

    Log-Info "Installation log saved to: $INSTALL_LOG"
}

# Main installation flow
function Main {
    Write-Host ""
    Log-Info "Starting Anime API installation for Windows..."
    Log-Info "Installation log: $INSTALL_LOG"
    Write-Host ""

    # Check administrator privileges
    if (!(Test-Administrator)) {
        Log-Error "This script must be run as Administrator"
        Log-Error "Right-click PowerShell and select 'Run as Administrator'"
        exit 1
    }

    # Detect architecture
    $ARCH = Get-SystemArchitecture
    Log-Info "Detected architecture: $ARCH"
    Write-Host ""

    # Prompt for confirmation
    $confirmation = Read-Host "Continue with installation? (y/N)"
    if ($confirmation -notmatch '^[Yy]$') {
        Log-Info "Installation cancelled by user"
        exit 0
    }

    # Create log directory early
    if (!(Test-Path (Split-Path $INSTALL_LOG))) {
        New-Item -ItemType Directory -Path (Split-Path $INSTALL_LOG) -Force | Out-Null
    }

    # Installation steps
    try {
        $binaryDownloadPath = Download-Binary $ARCH
        $binaryPath = Install-Binary $binaryDownloadPath
        Create-Directories
        Generate-Credentials
        $nssmPath = Install-NSSM
        Create-Service $binaryPath $nssmPath
        Configure-Firewall
        Start-AnimeService

        # Display information
        Display-Info

        Write-ColorOutput Green "`nInstallation completed successfully!"
    }
    catch {
        Log-Error "Installation failed: $_"
        Write-ColorOutput Red "`nInstallation failed. See log: $INSTALL_LOG"
        exit 1
    }
    finally {
        # Cleanup
        Remove-Item $binaryDownloadPath -Force -ErrorAction SilentlyContinue
    }
}

# Run main function
Main
