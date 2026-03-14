# ReqX Installer v1.1
$ErrorActionPreference = "Stop"

Write-Host "`n============================================================" -ForegroundColor Cyan
Write-Host "              🚀 ReqX: Smart Installer" -ForegroundColor Cyan
Write-Host "============================================================`n" -ForegroundColor Cyan

$exeName = "reqx.exe"
$destDir = "$env:USERPROFILE\.reqx\bin"
$repo = "aryanwalia2003/ReqX"
$zipName = "reqx-windows.zip"

try {
    # 1. Check for local binary (Dev mode)
    $localExe = Join-Path -Path $PWD -ChildPath $exeName
    if (-not (Test-Path $localExe)) {
        Write-Host "📡 Local binary not found. Searching GitHub Releases..." -ForegroundColor Gray
        
        $tempDir = Join-Path -Path $env:TEMP -ChildPath "reqx_install_$(Get-Random)"
        if (-not (Test-Path $tempDir)) { New-Item -ItemType Directory -Path $tempDir -Force | Out-Null }
        
        Write-Host "🔍 Connecting to GitHub API..." -ForegroundColor Gray
        $apiUri = "https://api.github.com/repos/$repo/releases/latest"
        
        # We must set a User-Agent or GitHub API might reject the request
        $apiResponse = Invoke-RestMethod -Uri $apiUri -Headers @{"User-Agent"="ReqX-Installer"}
        
        $asset = $apiResponse.assets | Where-Object { $_.name -like "*reqx-windows.zip*" }
        if (-not $asset) { throw "Could not find reqx-windows.zip in the latest release ($($apiResponse.tag_name))." }
        
        $downloadUrl = $asset.browser_download_url
        $zipPath = Join-Path -Path $tempDir -ChildPath $zipName
        
        Write-Host "📥 Downloading ReqX ($($apiResponse.tag_name))..." -ForegroundColor Cyan
        Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
        
        Write-Host "📦 Extracting package..." -ForegroundColor Gray
        Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force
        
        $found = Get-ChildItem -Path $tempDir -Filter $exeName -Recurse | Select-Object -First 1
        if (-not $found) { throw "reqx.exe missing after extraction." }
        $localExe = $found.FullName
    }

    # 2. Setup System Folder
    if (-not (Test-Path $destDir)) {
        New-Item -ItemType Directory -Path $destDir -Force | Out-Null
    }

    Write-Host "💾 Installing to $destDir..." -ForegroundColor Gray
    Copy-Item -Path $localExe -Destination "$destDir\$exeName" -Force
    Write-Host "✅ Installation successful!" -ForegroundColor Green

    # 3. PATH Management
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentPath -notlike "*$destDir*") {
        $sep = if ($currentPath -and -not $currentPath.EndsWith(";")) { ";" } else { "" }
        $newPath = if ([string]::IsNullOrEmpty($currentPath)) { $destDir } else { "$currentPath$sep$destDir" }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Host "🔗 Added ReqX to your User PATH." -ForegroundColor Yellow
    }

    Write-Host "`n✨ ReqX is now ready!" -ForegroundColor Green
    Write-Host "💡 Close this window and start a new PowerShell to use 'reqx'.`n" -ForegroundColor Cyan

} catch {
    Write-Host "`n❌ Installation Failed!" -ForegroundColor Red
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Host "Press Enter to exit..." -ForegroundColor Gray
Read-Host
return
