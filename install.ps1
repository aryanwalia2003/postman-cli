$exeName = "reqx.exe"
$destDir = "$env:USERPROFILE\.reqx\bin"
$configDir = "$env:USERPROFILE\.reqx"

Write-Host "--- ReqX Installer ---" -foregroundColor Cyan

# 1. Create Directories
if (-not (Test-Path $destDir)) {
    New-Item -ItemType Directory -Path $destDir -Force | Out-Null
    Write-Host "Created bin directory: $destDir"
}

# 2. Copy Executable
if (Test-Path $exeName) {
    Copy-Item -Path $exeName -Destination "$destDir\$exeName" -Force
    Write-Host "Copied $exeName to $destDir" -foregroundColor Green
} else {
    Write-Host "Error: $exeName not found in current directory!" -foregroundColor Red
    exit 1
}

# 3. Add to User PATH
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$destDir*") {
    $newPath = "$currentPath;$destDir"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "Added $destDir to User PATH." -foregroundColor Yellow
} else {
    Write-Host "Directory already in PATH."
}

Write-Host "--------------------------------------------------------"
Write-Host "Installation Complete!" -foregroundColor Green
Write-Host "RESTART your terminal and type 'reqx --help' to get started." -foregroundColor Yellow
Write-Host "--------------------------------------------------------"
