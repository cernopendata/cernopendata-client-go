#!/bin/bash

set -euo pipefail

REPO="cernopendata/cernopendata-client-go"
BINARY_NAME="cernopendata-client"
GITHUB_API_BASE_URL="${GITHUB_API_BASE_URL:-https://api.github.com}"
GITHUB_RELEASE_BASE_URL="${GITHUB_RELEASE_BASE_URL:-https://github.com}"

echo "Installing ${BINARY_NAME}..."

detect_os() {
    case "$(uname -s)" in
        Linux*)     OS=linux;;
        Darwin*)    OS=darwin;;
        *)          echo "Unsupported OS: $(uname -s)"; exit 1;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64)     ARCH=amd64;;
        arm64|aarch64) ARCH=arm64;;
        *)          echo "Unsupported architecture: $(uname -m)"; exit 1;;
    esac
}

detect_os
detect_arch

echo "Detected OS: ${OS}"
echo "Detected architecture: ${ARCH}"

get_latest_release() {
    curl -fsSL "${GITHUB_API_BASE_URL}/repos/${REPO}/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

LATEST_VERSION=$(get_latest_release)
if [ -z "$LATEST_VERSION" ]; then
    echo "Failed to fetch latest release version"
    exit 1
fi

echo "Latest version: ${LATEST_VERSION}"

BINARY_URL="${GITHUB_RELEASE_BASE_URL}/${REPO}/releases/download/${LATEST_VERSION}/${BINARY_NAME}-${OS}-${ARCH}"

echo "Downloading from: ${BINARY_URL}"

TEMP_DIR=$(mktemp -d)
trap 'rm -rf -- "${TEMP_DIR}"' EXIT

CHECKSUMS_URL="${GITHUB_RELEASE_BASE_URL}/${REPO}/releases/download/${LATEST_VERSION}/checksums.txt"
if ! curl -fsSL "${CHECKSUMS_URL}" -o "${TEMP_DIR}/checksums.txt"; then
    echo "Failed to download required checksums file"
    exit 1
fi
echo "Downloaded checksums file"

if ! curl -fsSL "${BINARY_URL}" -o "${TEMP_DIR}/${BINARY_NAME}"; then
    echo "Failed to download binary"
    exit 1
fi

verify_checksum() {
    if [ ! -f "${TEMP_DIR}/checksums.txt" ]; then
        echo "Required checksums file not found"
        exit 1
    fi

    BINARY_FILE="${BINARY_NAME}-${OS}-${ARCH}"

    if ! EXPECTED_CHECKSUM=$(awk -v target="${BINARY_FILE}" '
        NF == 0 { next }
        NF != 2 || length($1) != 64 || $1 !~ /^[[:xdigit:]]+$/ {
            printf "Malformed checksum entry on line %d\n", NR > "/dev/stderr"
            malformed = 1
            next
        }
        $2 == target {
            matches++
            checksum = tolower($1)
        }
        END {
            if (malformed) {
                exit 2
            }
            if (matches != 1) {
                printf "Expected exactly one checksum entry for %s, found %d\n", target, matches > "/dev/stderr"
                exit 3
            }
            print checksum
        }
    ' "${TEMP_DIR}/checksums.txt"); then
        echo "Checksum manifest validation failed"
        exit 1
    fi

    if command -v sha256sum >/dev/null 2>&1; then
        ACTUAL_CHECKSUM=$(sha256sum "${TEMP_DIR}/${BINARY_NAME}" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        ACTUAL_CHECKSUM=$(shasum -a 256 "${TEMP_DIR}/${BINARY_NAME}" | awk '{print $1}')
    else
        echo "sha256sum or shasum is required for checksum verification"
        exit 1
    fi

    ACTUAL_CHECKSUM=$(printf '%s' "${ACTUAL_CHECKSUM}" | tr '[:upper:]' '[:lower:]')

    if [ "$EXPECTED_CHECKSUM" != "$ACTUAL_CHECKSUM" ]; then
        echo "Checksum verification failed!"
        echo "Expected: ${EXPECTED_CHECKSUM}"
        echo "Actual:   ${ACTUAL_CHECKSUM}"
        exit 1
    fi

    echo "Checksum verified: ${ACTUAL_CHECKSUM}"
}

verify_checksum

chmod +x "${TEMP_DIR}/${BINARY_NAME}"

determine_install_dir() {
    if [ -w "/usr/local/bin" ]; then
        INSTALL_DIR="/usr/local/bin"
    elif [ -w "${HOME}/bin" ] || mkdir -p "${HOME}/bin" 2>/dev/null; then
        INSTALL_DIR="${HOME}/bin"
    elif [ -w "${HOME}/.local/bin" ] || mkdir -p "${HOME}/.local/bin" 2>/dev/null; then
        INSTALL_DIR="${HOME}/.local/bin"
    else
        echo "No writeable installation directory found. Please specify one with INSTALL_DIR environment variable."
        exit 1
    fi
}

if [ -z "${INSTALL_DIR:-}" ]; then
    determine_install_dir
fi

echo "Installing to: ${INSTALL_DIR}"
mkdir -p "${INSTALL_DIR}"
mv "${TEMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"

verify_installation() {
    if [ ! -x "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        echo "Installation verification failed"
        exit 1
    fi
    echo "Installation successful!"
    "${INSTALL_DIR}/${BINARY_NAME}" version
}

verify_installation

configure_path() {
    SHELL_NAME=$(basename "${SHELL:-sh}")
    PATH_ENTRY="export PATH=\"${INSTALL_DIR}:\$PATH\""

    case ":$PATH:" in
        *:"${INSTALL_DIR}":*)
            echo "${INSTALL_DIR} is already in PATH"
            return 0
            ;;
    esac

    echo "${INSTALL_DIR} is not in PATH"

    case "${SHELL_NAME}" in
        bash)
            CONFIG_FILE="${HOME}/.bashrc"
            ;;
        zsh)
            CONFIG_FILE="${HOME}/.zshrc"
            ;;
        fish)
            PATH_ENTRY="fish_add_path ${INSTALL_DIR}"
            CONFIG_FILE="${HOME}/.config/fish/config.fish"
            ;;
        *)
            CONFIG_FILE="${HOME}/.profile"
            ;;
    esac

    if [ ! -f "${CONFIG_FILE}" ] && [ "${SHELL_NAME}" != "fish" ]; then
        CONFIG_FILE="${HOME}/.profile"
    fi

    if grep -q "${INSTALL_DIR}" "${CONFIG_FILE}" 2>/dev/null; then
        echo "PATH entry already exists in ${CONFIG_FILE}"
        return 0
    fi

    echo "" >> "${CONFIG_FILE}"
    echo "# Added by ${BINARY_NAME} installer" >> "${CONFIG_FILE}"
    echo "${PATH_ENTRY}" >> "${CONFIG_FILE}"
    echo "Added ${INSTALL_DIR} to PATH in ${CONFIG_FILE}"
    echo "Please restart your shell or run: source ${CONFIG_FILE}"
}

configure_path

echo ""
echo "Installation complete!"
echo "You can now use: ${BINARY_NAME}"
