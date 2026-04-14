#!/bin/sh

set -eu

OWNER="srimon12"
REPO="qql-go"
API_URL="https://api.github.com/repos/$OWNER/$REPO/releases/latest"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

need_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "error: required command not found: $1" >&2
        exit 1
    fi
}

need_cmd curl
need_cmd tar
need_cmd mktemp

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
    Linux) os="linux" ;;
    Darwin) os="darwin" ;;
    *)
        echo "error: unsupported operating system: $os" >&2
        exit 1
        ;;
esac

case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *)
        echo "error: unsupported architecture: $arch" >&2
        exit 1
        ;;
esac

version="${VERSION:-}"
if [ -z "$version" ]; then
    version="$(curl -fsSL "$API_URL" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
fi

if [ -z "$version" ]; then
    echo "error: unable to determine release version" >&2
    exit 1
fi

version="${version#v}"
asset="qql-go_${version}_${os}_${arch}.tar.gz"
checksum_asset="qql-go_${version}_checksums.txt"
base_url="https://github.com/$OWNER/$REPO/releases/download/v$version"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT INT TERM

archive_path="$tmpdir/$asset"
checksum_path="$tmpdir/$checksum_asset"

echo "Downloading qql-go $version for $os/$arch..."
curl -fsSL "$base_url/$asset" -o "$archive_path"

if curl -fsSL "$base_url/$checksum_asset" -o "$checksum_path"; then
    if command -v sha256sum >/dev/null 2>&1; then
        (
            cd "$tmpdir"
            sha256sum -c "$checksum_asset" --ignore-missing >/dev/null
        )
    elif command -v shasum >/dev/null 2>&1; then
        expected="$(grep "  $asset\$" "$checksum_path" | awk '{print $1}')"
        if [ -n "$expected" ]; then
            actual="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
            if [ "$expected" != "$actual" ]; then
                echo "error: checksum verification failed for $asset" >&2
                exit 1
            fi
        fi
    fi
fi

mkdir -p "$INSTALL_DIR"
tar -xzf "$archive_path" -C "$tmpdir"
install_path="$INSTALL_DIR/qql-go"
cp "$tmpdir/qql-go" "$install_path"
chmod +x "$install_path"

case ":$PATH:" in
    *:"$INSTALL_DIR":*)
        path_note=""
        ;;
    *)
        path_note="Add $INSTALL_DIR to your PATH to run qql-go from any shell."
        ;;
esac

echo "Installed qql-go to $install_path"
if [ -n "$path_note" ]; then
    echo "$path_note"
fi
