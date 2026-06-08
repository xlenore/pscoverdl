#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="$ROOT_DIR/dist"
APP="pscoverdl"
CLI_MAIN="./cmd/pscoverdl"
GUI_MAIN="./cmd/pscoverdl-gui"
HOST_OS="$(go env GOHOSTOS)"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"
export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
export GOMODCACHE="${GOMODCACHE:-$ROOT_DIR/.cache/go-mod}"
mkdir -p "$GOCACHE" "$GOMODCACHE"

build_cli() {
    local goos="$1"
    local goarch="$2"
    local label="$3"
    local ext=""
    if [[ "$goos" == "windows" ]]; then
        ext=".exe"
    fi

    local out_dir="$DIST_DIR/$APP-$label"
    mkdir -p "$out_dir"
    echo "[build:cli] $label"
    (
        cd "$ROOT_DIR"
        CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "-s -w" -o "$out_dir/$APP$ext" "$CLI_MAIN"
    )
}

build_gui() {
    local goos="$1"
    local goarch="$2"
    local label="$3"
    local ext=""
    if [[ "$goos" == "windows" ]]; then
        ext=".exe"
    fi

    local out_dir="$DIST_DIR/$APP-$label"
    mkdir -p "$out_dir"
    echo "[build:gui] $label"
    (
        cd "$ROOT_DIR"
        CGO_ENABLED=1 GOOS="$goos" GOARCH="$goarch" go build -tags production -trimpath -ldflags "-s -w" -o "$out_dir/$APP-gui$ext" "$GUI_MAIN"
    )
}

build_cli darwin arm64 macos-arm64
build_cli darwin amd64 macos-x86_64
build_cli linux amd64 linux-x86_64
build_cli linux arm64 linux-aarch64
build_cli windows amd64 windows-x86_64
build_cli windows arm64 windows-aarch64

case "$HOST_OS" in
    darwin)
        build_gui darwin arm64 macos-arm64
        build_gui darwin amd64 macos-x86_64
        echo "[build:gui] skipped linux/windows Wails GUI: native CGO/WebView toolchains required"
        ;;
    linux)
        build_gui linux amd64 linux-x86_64
        build_gui linux arm64 linux-aarch64
        echo "[build:gui] skipped macos/windows Wails GUI: native CGO/WebView toolchains required"
        ;;
    windows)
        build_gui windows amd64 windows-x86_64
        build_gui windows arm64 windows-aarch64
        echo "[build:gui] skipped macos/linux Wails GUI: native CGO/WebView toolchains required"
        ;;
    *)
        echo "[build:gui] skipped: unsupported host $HOST_OS"
        ;;
esac

echo "[build] binaries written to $DIST_DIR"
