#!/usr/bin/env sh
set -eu

REPO="MohamedElashri/agrep"
DEFAULT_VERSION="v0.1.0"
VERSION="${VERSION:-$DEFAULT_VERSION}"

# Normalize version tag
case "$VERSION" in
  v*) TAG="$VERSION" ; VER="${VERSION#v}" ;;
  *) TAG="v$VERSION" ; VER="$VERSION" ;;
esac

# Detect OS
OS_RAW="$(uname -s)"
case "$OS_RAW" in
  Linux) OS="linux" ;;
  Darwin) OS="darwin" ;;
  FreeBSD) OS="freebsd" ;;
  OpenBSD) OS="openbsd" ;;
  NetBSD) OS="netbsd" ;;
  DragonFly) OS="dragonfly" ;;
  *)
    echo "Error: Unsupported operating system '$OS_RAW'." >&2
    exit 1
    ;;
esac

# Detect Architecture
ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Error: Unsupported architecture '$ARCH_RAW'." >&2
    exit 1
    ;;
esac

if [ "$OS" = "dragonfly" ] && [ "$ARCH" = "arm64" ]; then
  echo "Error: DragonFly BSD does not support arm64 builds." >&2
  exit 1
fi

TARBALL="agrep_${VER}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${TARBALL}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"

# Select install destination directory
if [ -n "${BINDIR:-}" ]; then
  TARGET_DIR="$BINDIR"
elif [ -w "/usr/local/bin" ]; then
  TARGET_DIR="/usr/local/bin"
elif [ -d "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
  TARGET_DIR="$HOME/.local/bin"
else
  TARGET_DIR="/usr/local/bin"
fi

printf "Installing agrep %s (%s/%s)...\n" "$TAG" "$OS" "$ARCH"

# Create a temporary workspace
TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'agrep-install')"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

download_file() {
  url="$1"
  dest="$2"
  if [ -n "${CUSTOM_DOWNLOADER:-}" ]; then
    $CUSTOM_DOWNLOADER "$url" "$dest"
  elif command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    echo "Error: neither curl nor wget is available." >&2
    exit 1
  fi
}

# Download release archive and checksum file
download_file "$DOWNLOAD_URL" "$TMP_DIR/$TARBALL"
download_file "$CHECKSUMS_URL" "$TMP_DIR/checksums.txt"

# Verify SHA-256 hash
EXPECTED_HASH="$(grep "  ${TARBALL}\$" "$TMP_DIR/checksums.txt" | awk '{print $1}')"
if [ -z "$EXPECTED_HASH" ]; then
  echo "Error: Archive '${TARBALL}' not found in checksums.txt" >&2
  exit 1
fi

compute_hash() {
  target="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$target" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$target" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$target" | awk '{print $NF}'
  else
    echo "Error: no SHA-256 tool found (need sha256sum, shasum, or openssl)." >&2
    exit 1
  fi
}

ACTUAL_HASH="$(compute_hash "$TMP_DIR/$TARBALL")"
if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
  echo "Error: Checksum mismatch for ${TARBALL}!" >&2
  echo "Expected: $EXPECTED_HASH" >&2
  echo "Actual:   $ACTUAL_HASH" >&2
  exit 1
fi

# Extract and install binary
tar -xzf "$TMP_DIR/$TARBALL" -C "$TMP_DIR"
if [ ! -f "$TMP_DIR/agrep" ]; then
  echo "Error: 'agrep' binary not found in release archive." >&2
  exit 1
fi

mkdir -p "$TARGET_DIR" 2>/dev/null || true

if [ -w "$TARGET_DIR" ]; then
  install -m 755 "$TMP_DIR/agrep" "$TARGET_DIR/agrep" 2>/dev/null || cp "$TMP_DIR/agrep" "$TARGET_DIR/agrep" && chmod 755 "$TARGET_DIR/agrep"
else
  echo "Escalating permissions with sudo to write to $TARGET_DIR..."
  sudo cp "$TMP_DIR/agrep" "$TARGET_DIR/agrep"
  sudo chmod 755 "$TARGET_DIR/agrep"
fi

printf "\nSuccessfully installed agrep %s to %s/agrep\n" "$TAG" "$TARGET_DIR"

# Warn if target directory is not in PATH
case ":$PATH:" in
  *":$TARGET_DIR:"*) ;;
  *)
    printf "\nNote: %s is not currently in your PATH.\n" "$TARGET_DIR"
    printf "You can add it by running:\n  export PATH=\"%s:\$PATH\"\n" "$TARGET_DIR"
    ;;
esac
