# Build script for native Windows ARM64 → target AMD64.
# Usage (from backend/): .\scripts\build-windows.ps1
# Requires:
#   - Go, Wails CLI, Node.js — all in PATH
#   - MSYS2 with AMD64 toolchain:
#       pacman -S mingw-w64-x86_64-gcc
#       Add C:\msys64\mingw64\bin to PATH

$ErrorActionPreference = "Stop"

$PANDOC_VERSION = "3.6.4"
$XPDF_VERSION   = "4.06"

# Resolve paths relative to backend/ (parent of this script), not .NET working dir.
$ROOT = Split-Path $PSScriptRoot -Parent
$BIN  = Join-Path $ROOT "bin"

New-Item -ItemType Directory -Force -Path $BIN | Out-Null
Add-Type -AssemblyName System.IO.Compression.FileSystem

# --- pandoc.exe ---
if (-not (Test-Path (Join-Path $BIN "pandoc.exe"))) {
    Write-Host "Downloading pandoc $PANDOC_VERSION..."
    $url = "https://github.com/jgm/pandoc/releases/download/$PANDOC_VERSION/pandoc-$PANDOC_VERSION-windows-x86_64.zip"
    $tmp = "$env:TEMP\pandoc-win-$([System.IO.Path]::GetRandomFileName()).zip"
    Invoke-WebRequest -Uri $url -OutFile $tmp
    $zip   = [System.IO.Compression.ZipFile]::OpenRead($tmp)
    $entry = $zip.Entries | Where-Object { $_.Name -eq "pandoc.exe" } | Select-Object -First 1
    [System.IO.Compression.ZipFileExtensions]::ExtractToFile($entry, (Join-Path $BIN "pandoc.exe"), $true)
    $zip.Dispose()
    Remove-Item $tmp
    Write-Host "pandoc $PANDOC_VERSION ready at bin\pandoc.exe"
}

# --- pdftotext.exe ---
if (-not (Test-Path (Join-Path $BIN "pdftotext.exe"))) {
    Write-Host "Downloading pdftotext (XPDF $XPDF_VERSION)..."
    $url = "https://dl.xpdfreader.com/xpdf-tools-win-$XPDF_VERSION.zip"
    $tmp = "$env:TEMP\xpdf-win-$([System.IO.Path]::GetRandomFileName()).zip"
    Invoke-WebRequest -Uri $url -OutFile $tmp
    $zip   = [System.IO.Compression.ZipFile]::OpenRead($tmp)
    $entry = $zip.Entries | Where-Object { $_.FullName -like "*/bin64/pdftotext.exe" } | Select-Object -First 1
    [System.IO.Compression.ZipFileExtensions]::ExtractToFile($entry, (Join-Path $BIN "pdftotext.exe"), $true)
    $zip.Dispose()
    Remove-Item $tmp
    Write-Host "pdftotext XPDF $XPDF_VERSION ready at bin\pdftotext.exe"
}

# --- Build ---
Set-Location $ROOT
Write-Host "Building GnJ Windows AMD64 installer..."
$env:GOARCH = "amd64"
$env:CC     = "x86_64-w64-mingw32-gcc"
wails build --platform windows/amd64 -nsis
Write-Host "Done: build\bin\GnJ-amd64-installer.exe"
