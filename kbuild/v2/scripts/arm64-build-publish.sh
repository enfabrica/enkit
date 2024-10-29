#!/bin/sh

# Build and publish arm64 kernel artifacts for a particular configuration

set -e

SCRIPT_PATH="$(dirname $(realpath $0))"

ARCH=$1
FLAVOUR=$2
KERNEL_SRC="$3"
KERNEL_VERSION="$4"
BUILD_ROOT="$5"
ASTORE_LABEL="$6"
ASTORE_BASE="$7"
ASTORE_META_DIR="$8"

TARGET="${ARCH}-${FLAVOUR}"
KERNEL_BUILD_DIR="$BUILD_ROOT/kbuild/${TARGET}"
OUTPUT_KRELEASE_DIR="$BUILD_ROOT/krelease/${TARGET}"

# Creates a tarball of build artifacts for each kernel spec:
# - kernel version string
# - kernel config file
# - kernel out of tree module build artifacts
# - kernel Image
${SCRIPT_PATH}/build-kernel-release.sh \
     "$KERNEL_SRC"       \
     "$KERNEL_BUILD_DIR" \
     "$KERNEL_VERSION"   \
     "$ARCH"             \
     "$FLAVOUR"          \
     "$OUTPUT_KRELEASE_DIR"
