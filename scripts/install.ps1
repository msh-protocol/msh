# msh installer for Windows PowerShell
# Usage: irm https://raw.githubusercontent.com/msh-protocol/msh/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

$repo = "msh-protocol/msh"
$installDir = "$HOME\.msh\bin"

Write-Host "==> Detecting system architecture..." -ForegroundColor Cyan
$arch = if ([System.Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Error "32-bit Windows is not supported."
}

Write-Host "==> Finding latest release of msh..." -ForegroundColor Cyan
$latestTag = "v1.3.0"
try {
    $releaseJson = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -UseBasicParsing
    if ($releaseJson.tag_name) {
        $latestTag = $releaseJson.tag_name
    }
} catch {
    # fallback to default tag
}

$filename = "msh-$latestTag-windows-$arch.zip"
$downloadUrl = "https://github.com/$repo/releases/download/$latestTag/$filename"

$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
    Write-Host "==> Downloading msh $latestTag ($arch)..." -ForegroundColor Cyan
    $zipPath = Join-Path $tmpDir $filename
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing

    Write-Host "==> Extracting to $installDir..." -ForegroundColor Cyan
    if (!(Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    }
    Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force
    Copy-Item -Path (Join-Path $tmpDir "msh.exe") -Destination (Join-Path $installDir "msh.exe") -Force

    # Add to User PATH if missing
    $userPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$installDir*") {
        Write-Host "==> Adding $installDir to User PATH..." -ForegroundColor Cyan
        [System.Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
        $env:Path = "$env:Path;$installDir"
    }

    Write-Host "==> msh installed successfully!" -ForegroundColor Green
    & (Join-Path $installDir "msh.exe") version
    Write-Host ""
    Write-Host "Try it now in a new terminal window:" -ForegroundColor Yellow
    Write-Host "  msh git status" -ForegroundColor Yellow
    Write-Host "  msh npm test" -ForegroundColor Yellow
} finally {
    Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
