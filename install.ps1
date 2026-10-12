# clavis installer for Windows (PowerShell).
#
#   irm https://raw.githubusercontent.com/mutex-solutions/clavis/main/install.ps1 | iex
#
# Env overrides:
#   $env:CLAVIS_VERSION = "v0.1.0"   install a specific tag (default: latest)
#   $env:CLAVIS_INSTALL_DIR = "..."  where to put the binary

$ErrorActionPreference = "Stop"
$Repo = "mutex-solutions/clavis"

# --- Detect arch ---
$arch = if ([Environment]::Is64BitOperatingSystem) {
  if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else { throw "unsupported architecture" }

# --- Resolve version ---
$version = $env:CLAVIS_VERSION
if (-not $version) {
  Write-Host "  Finding the latest release..."
  $rel = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "clavis-installer" }
  $version = $rel.tag_name
}
if (-not $version) { throw "could not determine latest version" }

$archive = "clavis_windows_$arch.zip"
$url = "https://github.com/$Repo/releases/download/$version/$archive"

# --- Install dir ---
$installDir = $env:CLAVIS_INSTALL_DIR
if (-not $installDir) { $installDir = Join-Path $env:LOCALAPPDATA "Programs\clavis" }
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

# --- Download + extract ---
$tmp = New-Item -ItemType Directory -Force -Path (Join-Path $env:TEMP ("clavis-" + [guid]::NewGuid()))
try {
  Write-Host "  Downloading clavis $version (windows/$arch)..."
  $zip = Join-Path $tmp $archive
  Invoke-WebRequest -Uri $url -OutFile $zip -Headers @{ "User-Agent" = "clavis-installer" }
  Expand-Archive -Path $zip -DestinationPath $tmp -Force
  Copy-Item -Path (Join-Path $tmp "clavis.exe") -Destination (Join-Path $installDir "clavis.exe") -Force
  Write-Host "  Installed to $installDir\clavis.exe"

  # --- PATH hint ---
  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host "  Added $installDir to your user PATH (restart your shell)."
  }
  Write-Host ""
  Write-Host '  Run "clavis add" to create your first profile.'
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
