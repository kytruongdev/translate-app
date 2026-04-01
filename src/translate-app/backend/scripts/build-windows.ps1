# Build script for native Windows.
# Usage (from backend/): .\scripts\build-windows.ps1
# Requires: Go, Wails CLI, Node.js (all in PATH)

$ErrorActionPreference = "Stop"

$PANDOC_VERSION = "3.6.4"
$XPDF_VERSION   = "4.06"

New-Item -ItemType Directory -Force -Path bin | Out-Null

# --- pandoc.exe ---
if (-not (Test-Path "bin\pandoc.exe")) {
    Write-Host "Downloading pandoc $PANDOC_VERSION..."
    $url = "https://github.com/jgm/pandoc/releases/download/$PANDOC_VERSION/pandoc-$PANDOC_VERSION-windows-x86_64.zip"
    $tmp = "$env:TEMP\pandoc-win.zip"
    Invoke-WebRequest -Uri $url -OutFile $tmp
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($tmp)
    $entry = $zip.Entries | Where-Object { $_.Name -eq "pandoc.exe" } | Select-Object -First 1
    [System.IO.Compression.ZipFileExtensions]::ExtractToFile($entry, "bin\pandoc.exe", $true)
    $zip.Dispose()
    Remove-Item $tmp
    Write-Host "pandoc $PANDOC_VERSION ready at bin\pandoc.exe"
}

# --- pdftotext.exe ---
if (-not (Test-Path "bin\pdftotext.exe")) {
    Write-Host "Downloading pdftotext (XPDF $XPDF_VERSION)..."
    $url = "https://dl.xpdfreader.com/xpdf-tools-win-$XPDF_VERSION.zip"
    $tmp = "$env:TEMP\xpdf-win.zip"
    Invoke-WebRequest -Uri $url -OutFile $tmp
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($tmp)
    $entry = $zip.Entries | Where-Object { $_.FullName -like "*/bin64/pdftotext.exe" } | Select-Object -First 1
    [System.IO.Compression.ZipFileExtensions]::ExtractToFile($entry, "bin\pdftotext.exe", $true)
    $zip.Dispose()
    Remove-Item $tmp
    Write-Host "pdftotext XPDF $XPDF_VERSION ready at bin\pdftotext.exe"
}

# --- Build ---
Write-Host "Building GnJ Windows installer..."
wails build --platform windows/amd64 -nsis
Write-Host "Done: build\bin\GnJ-amd64-installer.exe"
