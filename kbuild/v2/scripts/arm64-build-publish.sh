#!/bin/sh

# Build and publish arm64 kernel artifacts for a particular configuration

set -ex

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
BUILD_DEB_DIR="$BUILD_ROOT/deb-build/${TARGET}"
OUTPUT_DEB_DIR="$BUILD_ROOT/deb-out/${TARGET}"
OUTPUT_REPO_DIR="$BUILD_ROOT/apt-repo/${TARGET}"
OUTPUT_BAZEL_ARCHIVE_DIR="$BUILD_ROOT/bazel-archive/${TARGET}"
OUTPUT_APT_ARCHIVE_DIR="$BUILD_ROOT/deb-archive/${TARGET}"

echo "PKG_CONFIG_PATH=$PKG_CONFIG_PATH"

dpkg --print-architecture
# amd64

dpkg --print-foreign-architectures
# arm64

dpkg --add-architecture arm64

dpkg --print-architecture
# amd64

dpkg --print-foreign-architectures
# arm64

apt update
apt install -yV gcc-aarch64-linux-gnu \
g++-aarch64-linux-gnu \
libpci-dev

# export PKG_CONFIG_PATH=/usr/lib/aarch64-linux-gnu/pkgconfig:$PKG_CONFIG_PATH

# Builds the .deb kernel packages for arch, flavour
${SCRIPT_PATH}/build-debs.sh "$KERNEL_SRC" "$KERNEL_VERSION" "$ARCH" "$FLAVOUR" "$BUILD_DEB_DIR" "$OUTPUT_DEB_DIR"

echo Done.
exit 1

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

# Uploads the bazel ready kernel build tarballs
${SCRIPT_PATH}/upload-kernel-build.sh \
     "$KERNEL_BUILD_DIR"          \
     "$OUTPUT_KRELEASE_DIR"       \
     "$ARCH"                      \
     "$FLAVOUR"                   \
     "$ASTORE_BASE"               \
     "$ASTORE_META_DIR"

# Generate Bazel include file from upload meta-data files
${SCRIPT_PATH}/gen-bazel-meta2.sh \
     "$KERNEL_BUILD_DIR"          \
     "$OUTPUT_KRELEASE_DIR"       \
     "$ARCH"                      \
     "$FLAVOUR"                   \
     "$ASTORE_BASE"               \
     "$ASTORE_META_DIR"           \
     "$ASTORE_LABEL"
