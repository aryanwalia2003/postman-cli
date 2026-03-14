$exeName = "reqx.exe"
$destDir = "$env:USERPROFILE\.reqx\bin"
$repo = "aryanwalia2003/ReqX"
$zipName = "reqx-windows.zip"

Write-Host "`n--- 🚀 ReqX Smart Installer ---" -foregroundColor Cyan

# 1. Detect Execution Mode & Get Binary
$localExe = Join-Path (Get-Location) $exeName
if (-not (Test-Path $localExe)) {
    Write-Host "📡 Local binary not found. Initiating web-based installation..." -foregroundColor Gray
    
    $tempDir = Join-Path $env:TEMP "reqx_install_$(Get-Random)"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    
    try {
        Write-Host "🔍 Fetching latest release info from GitHub..." -foregroundColor Gray
        $apiInfo = iwr -useb "https://api.github.com/repos/$repo/releases/latest" | ConvertFrom-Json
        $asset = $apiInfo.assets | Where-Object { $_.name -eq $zipName }
        
        if (-not $asset) { throw "Could not find $zipName in the latest release." }
        
        $downloadUrl = $asset.browser_download_url
        $zipPath = Join-Path $tempDir $zipName
        
        Write-Host "📥 Downloading ReqX ($($apiInfo.tag_name))..." -foregroundColor Cyan
        iwr -useb $downloadUrl -OutFile $zipPath
        
        Write-Host "📦 Extracting package..." -foregroundColor Gray
        Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force
        
        # Binary might be inside a 'dist' folder in the zip
        $extractedExe = Get-ChildItem -Path $tempDir -Filter $exeName -Recurse | Select-Object -First 1
        if (-not $extractedExe) { throw "reqx.exe not found in downloaded package." }
        
        $localExe = $extractedExe.FullName
    } catch {
        Write-Host "❌ Error during web install: $_" -foregroundColor Red
        return
    }
}

# 2. Create Destination Directories
if (-not (Test-Path $destDir)) {
    New-Item -ItemType Directory -Path $destDir -Force | Out-Null
}

# 3. Copy/Install Binary
try {
    Write-Host "💾 Installing ReqX to $destDir..." -foregroundColor Gray
    Copy-Item -Path $localExe -Destination "$destDir\$exeName" -Force
    Write-Host "✅ Binary installed successfully!" -foregroundColor Green
} catch {
    Write-Host "❌ Failed to copy binary: $_" -foregroundColor Red
    return
}

# 4. Update User PATH
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$destDir*") {
    $newPath = "$currentPath;$destDir"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "🔗 Added ReqX to your User PATH." -foregroundColor Yellow
}

Write-Host "`n" + ("=" * 50)
Write-Host " 🎉 ReqX is now installed!" -foregroundColor Green
Write-Host " 💡 RESTART your terminal to start using 'reqx'." -foregroundColor Cyan
Write-Host ("=" * 50) + "`n"
