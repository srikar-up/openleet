# OpenLeet Windows Installer
# Usage: irm https://raw.githubusercontent.com/srikar-up/openleet/main/install.ps1 | iex
# Or run locally from a clone: .\install.ps1

$ErrorActionPreference = "Stop"

$Repo = "srikar-up/openleet"
$InstallDir = Join-Path $env:LOCALAPPDATA "openleet\bin"

Write-Host @"
  ___                    _                 _
 / _ \ _ __   ___ _ __ | |    ___  ___  _| |_
| | | | '_ \ / _ \ '_ \| |   / _ \/ _ \/ _   _|
| |_| | |_) |  __/ | | | |__|  __/  __/ (_| |_
 \___/| .__/ \___|_| |_|_____\___|\___|\__|\__|
      |_|
  Distraction-free offline DSA practice
"@ -ForegroundColor Cyan

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

Write-Host "Installing OpenLeet to: $InstallDir" -ForegroundColor Gray

$TargetExe = Join-Path $InstallDir "openleet.exe"
$SourceExe = $null

# Prefer a local build so contributors can install without a release.
if ($PSScriptRoot) {
    $candidate = Join-Path $PSScriptRoot "bin\openleet.exe"
    if (Test-Path $candidate) { $SourceExe = $candidate }
}

if (-not $SourceExe) {
    $repoRoot = $PSScriptRoot
    if (-not $repoRoot -or -not (Test-Path (Join-Path $repoRoot "cmd\openleet\main.go"))) {
        if (Test-Path ".\cmd\openleet\main.go") { $repoRoot = (Get-Location).Path } else { $repoRoot = $null }
    }
    if ($repoRoot -and (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "Building OpenLeet with the local Go toolchain..." -ForegroundColor Yellow
        Push-Location $repoRoot
        try {
            go build -o $TargetExe ./cmd/openleet
        } finally {
            Pop-Location
        }
        $SourceExe = $TargetExe
    }
}

if (-not $SourceExe) {
    Write-Host "Downloading the latest Windows release..." -ForegroundColor Yellow

    $headers = @{ "User-Agent" = "openleet-installer" }
    $releaseUrl = if ($env:OPENLEET_VERSION) {
        "https://api.github.com/repos/$Repo/releases/tags/$($env:OPENLEET_VERSION)"
    } else {
        "https://api.github.com/repos/$Repo/releases/latest"
    }
    $release = Invoke-RestMethod -Uri $releaseUrl -Headers $headers

    $asset = $release.assets | Where-Object { $_.name -like "openleet_Windows_*.zip" } | Select-Object -First 1
    if (-not $asset) {
        throw "No Windows release asset found in $($release.tag_name). Install Go and run this script from a clone instead."
    }

    $tempDir = Join-Path $env:TEMP ("openleet-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    try {
        $zipPath = Join-Path $tempDir $asset.name
        Invoke-WebRequest -Uri $asset.browser_download_url -Headers $headers -OutFile $zipPath

        $checksumAsset = $release.assets | Where-Object { $_.name -eq "checksums.txt" } | Select-Object -First 1
        if ($checksumAsset) {
            $checksumPath = Join-Path $tempDir "checksums.txt"
            Invoke-WebRequest -Uri $checksumAsset.browser_download_url -Headers $headers -OutFile $checksumPath
            $expected = (Get-Content $checksumPath | Where-Object { $_ -match [regex]::Escape($asset.name) } | Select-Object -First 1).Split()[0]
            $actual = (Get-FileHash $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($expected -and $actual -ne $expected.ToLowerInvariant()) {
                throw "Checksum verification failed for $($asset.name)."
            }
        }

        Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force
        $extracted = Get-ChildItem $tempDir -Recurse -Filter "openleet.exe" | Select-Object -First 1
        if (-not $extracted) { throw "The release archive did not contain openleet.exe." }
        Copy-Item $extracted.FullName $TargetExe -Force
        $SourceExe = $TargetExe
    } finally {
        Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

if ($SourceExe -and $SourceExe -ne $TargetExe) {
    Copy-Item $SourceExe $TargetExe -Force
}

$userPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
if (-not $userPath) { $userPath = "" }
if ($userPath -split ";" -notcontains $InstallDir) {
    Write-Host "Adding $InstallDir to the user PATH..." -ForegroundColor Green
    $newPath = if ($userPath) { "$InstallDir;$userPath" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $newPath, [EnvironmentVariableTarget]::User)
    $env:Path = "$InstallDir;$env:Path"
} else {
    Write-Host "$InstallDir is already on the user PATH." -ForegroundColor Gray
}

Write-Host @"

========================================================================
 OpenLeet installed successfully.
========================================================================
 Run:
   openleet

 Try:
   openleet --view roadmap
   openleet --list-themes

 Documentation: https://github.com/$Repo#readme
========================================================================
"@ -ForegroundColor Green
