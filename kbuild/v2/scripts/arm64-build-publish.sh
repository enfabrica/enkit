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

# BTF: .tmp_vmlinux.btf: pahole (pahole) is not available
# Failed to generate BTF for vmlinux
# Try to disable CONFIG_DEBUG_INFO_BTF
# make[3]: *** [/builder/home/scratch-arm64/kernel-builder/deb-build/arm64-generic/source/scripts/Makefile.vmlinux:35: vmlinux] Error 1
# make[2]: *** [/builder/home/scratch-arm64/kernel-builder/deb-build/arm64-generic/source/Makefile:1249: vmlinux] Error 2
# make[2]: *** Waiting for unfinished jobs....

# Builds the .deb kernel packages for arch, flavour
${SCRIPT_PATH}/build-debs.sh "$KERNEL_SRC" "$KERNEL_VERSION" "$ARCH" "$FLAVOUR" "$BUILD_DEB_DIR" "$OUTPUT_DEB_DIR"

# Creates a portable Debian APT repository for arch, flavour
${SCRIPT_PATH}/repo-deb.sh "$OUTPUT_DEB_DIR" "$ARCH" "$FLAVOUR" "$OUTPUT_REPO_DIR"

# Creates a bazel ready tarball for building kernel modules
${SCRIPT_PATH}/archive-bazel-deb.sh "$OUTPUT_DEB_DIR" "$ARCH" "$FLAVOUR" "$OUTPUT_BAZEL_ARCHIVE_DIR"

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
