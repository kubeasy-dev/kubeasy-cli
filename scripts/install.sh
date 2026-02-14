#!/bin/sh
# Kubeasy CLI installer
# Usage: curl -fsSL https://download.kubeasy.dev/install.sh | sh
#
# Environment variables:
#   KUBEASY_VERSION     - specific version to install (e.g. v2.5.0), defaults to latest
#   KUBEASY_INSTALL_DIR - installation directory, defaults to /usr/local/bin or ~/.local/bin

set -e

DOWNLOAD_BASE="https://download.kubeasy.dev"
PROJECT_NAME="kubeasy-cli"

main() {
    os="$(detect_os)"
    arch="$(detect_arch)"
    version="$(resolve_version)"
    install_dir="$(resolve_install_dir)"

    echo "Installing kubeasy ${version} (${os}/${arch}) to ${install_dir}"

    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    archive_name="${PROJECT_NAME}_${version}_${os}_${arch}.tar.gz"
    url="${DOWNLOAD_BASE}/${PROJECT_NAME}/${version}/${archive_name}"

    echo "Downloading ${url}..."
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "${tmpdir}/kubeasy.tar.gz"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "${tmpdir}/kubeasy.tar.gz" "$url"
    else
        echo "Error: curl or wget is required" >&2
        exit 1
    fi

    tar -xzf "${tmpdir}/kubeasy.tar.gz" -C "$tmpdir"

    if [ ! -f "${tmpdir}/kubeasy" ]; then
        echo "Error: binary not found in archive" >&2
        exit 1
    fi

    mkdir -p "$install_dir"

    if [ -w "$install_dir" ]; then
        cp "${tmpdir}/kubeasy" "${install_dir}/kubeasy"
        chmod +x "${install_dir}/kubeasy"
    else
        echo "Elevated permissions required to install to ${install_dir}"
        sudo cp "${tmpdir}/kubeasy" "${install_dir}/kubeasy"
        sudo chmod +x "${install_dir}/kubeasy"
    fi

    echo "kubeasy ${version} installed to ${install_dir}/kubeasy"

    check_path "$install_dir"
}

detect_os() {
    uname_s="$(uname -s)"
    case "$uname_s" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)
            echo "Error: unsupported OS: ${uname_s}" >&2
            exit 1
            ;;
    esac
}

detect_arch() {
    uname_m="$(uname -m)"
    case "$uname_m" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *)
            echo "Error: unsupported architecture: ${uname_m}" >&2
            exit 1
            ;;
    esac
}

resolve_version() {
    if [ -n "$KUBEASY_VERSION" ]; then
        echo "$KUBEASY_VERSION"
        return
    fi

    latest_url="${DOWNLOAD_BASE}/latest"
    if command -v curl >/dev/null 2>&1; then
        version="$(curl -fsSL "$latest_url")"
    elif command -v wget >/dev/null 2>&1; then
        version="$(wget -qO- "$latest_url")"
    else
        echo "Error: curl or wget is required" >&2
        exit 1
    fi

    version="$(echo "$version" | tr -d '[:space:]')"

    if [ -z "$version" ]; then
        echo "Error: could not resolve latest version" >&2
        exit 1
    fi

    echo "$version"
}

resolve_install_dir() {
    if [ -n "$KUBEASY_INSTALL_DIR" ]; then
        echo "$KUBEASY_INSTALL_DIR"
        return
    fi

    if [ -w "/usr/local/bin" ] || [ "$(id -u)" = "0" ]; then
        echo "/usr/local/bin"
    else
        echo "${HOME}/.local/bin"
    fi
}

check_path() {
    dir="$1"
    case ":${PATH}:" in
        *":${dir}:"*) ;;
        *)
            echo ""
            echo "WARNING: ${dir} is not in your PATH."
            echo "Add it by running:"
            echo "  export PATH=\"${dir}:\$PATH\""
            echo ""
            echo "To make it permanent, add the line above to your shell profile (~/.bashrc, ~/.zshrc, etc.)"
            ;;
    esac
}

main
