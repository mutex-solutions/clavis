#!/bin/sh
# clavis installer — downloads the latest release for your OS/arch.
#
#   curl -fsSL https://raw.githubusercontent.com/dinhlongviolin1/clavis/main/install.sh | sh
#
# Env overrides:
#   CLAVIS_VERSION=v0.1.0   install a specific tag (default: latest)
#   CLAVIS_INSTALL_DIR=~/.local/bin   where to put the binary
set -eu

REPO="dinhlongviolin1/clavis"
BINARY="clavis"

info()  { printf '  %s\n' "$1"; }
err()   { printf 'error: %s\n' "$1" >&2; exit 1; }

# --- Detect OS ---
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux)  os=linux ;;
  darwin) os=darwin ;;
  *) err "unsupported OS: $os (Windows: download the .zip from the releases page)" ;;
esac

# --- Detect arch ---
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) err "unsupported architecture: $arch" ;;
esac

# --- Resolve version ---
version="${CLAVIS_VERSION:-}"
if [ -z "$version" ]; then
  info "Finding the latest release…"
  version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
  [ -n "$version" ] || err "could not determine latest version (is a release published?)"
fi

archive="${BINARY}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$version/$archive"

# --- Install dir ---
install_dir="${CLAVIS_INSTALL_DIR:-}"
if [ -z "$install_dir" ]; then
  if [ -w "/usr/local/bin" ] 2>/dev/null; then
    install_dir="/usr/local/bin"
  else
    install_dir="$HOME/.local/bin"
  fi
fi
mkdir -p "$install_dir"

# --- Download + extract ---
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
info "Downloading clavis $version ($os/$arch)…"
curl -fsSL "$url" -o "$tmp/$archive" || err "download failed: $url"

# Verify checksum if the release ships one.
if curl -fsSL "https://github.com/$REPO/releases/download/$version/checksums.txt" -o "$tmp/checksums.txt" 2>/dev/null; then
  if command -v sha256sum >/dev/null 2>&1; then
    ( cd "$tmp" && grep " $archive\$" checksums.txt | sha256sum -c - >/dev/null 2>&1 ) \
      && info "Checksum verified." || info "Checksum not verified (skipping)."
  fi
fi

tar -xzf "$tmp/$archive" -C "$tmp" || err "extract failed"
[ -f "$tmp/$BINARY" ] || err "binary not found in archive"

install -m 0755 "$tmp/$BINARY" "$install_dir/$BINARY" 2>/dev/null \
  || { mv "$tmp/$BINARY" "$install_dir/$BINARY" && chmod 0755 "$install_dir/$BINARY"; }

info "Installed to $install_dir/$BINARY"

# --- PATH hint ---
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) printf '\n  Add %s to your PATH:\n    export PATH="%s:$PATH"\n' "$install_dir" "$install_dir" ;;
esac

printf '\n  Run "clavis add" to create your first profile.\n'
