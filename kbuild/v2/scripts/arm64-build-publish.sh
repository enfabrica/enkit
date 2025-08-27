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

if [ "$FLAVOUR" = "generic" ] ; then

BUILD_DEB_DIR="$BUILD_ROOT/deb-build/${TARGET}"
OUTPUT_DEB_DIR="$BUILD_ROOT/deb-out/${TARGET}"
OUTPUT_REPO_DIR="$BUILD_ROOT/apt-repo/${TARGET}"
OUTPUT_BAZEL_ARCHIVE_DIR="$BUILD_ROOT/bazel-archive/${TARGET}"
OUTPUT_APT_ARCHIVE_DIR="$BUILD_ROOT/deb-archive/${TARGET}"

echo "PKG_CONFIG_PATH=$PKG_CONFIG_PATH"

# Builds the .deb kernel packages for arch, flavour
${SCRIPT_PATH}/build-debs.sh "$KERNEL_SRC" "$KERNEL_VERSION" "$ARCH" "$FLAVOUR" "$BUILD_DEB_DIR" "$OUTPUT_DEB_DIR"

find "$OUTPUT_DEB_DIR" >> /workspace/${TARGET}.done
while [ $(ls /workspace/*done | wc -l) -lt 2 ] ; do
    sleep 10
done
echo "Done with build-debs.sh"
exit 0

# Creates a portable Debian APT repository for arch, flavour
${SCRIPT_PATH}/repo-deb.sh "$OUTPUT_DEB_DIR" "$ARCH" "$FLAVOUR" "$OUTPUT_REPO_DIR"

# Creates a bazel ready tarball for building kernel modules
${SCRIPT_PATH}/archive-bazel-deb.sh "$OUTPUT_DEB_DIR" "$ARCH" "$FLAVOUR" "$OUTPUT_BAZEL_ARCHIVE_DIR"

# Creates a tarball of a Debian APT repository for arch, flavour
${SCRIPT_PATH}/archive-deb.sh "$OUTPUT_DEB_DIR" "$OUTPUT_REPO_DIR" "$ARCH" "$FLAVOUR" "$OUTPUT_APT_ARCHIVE_DIR"

echo "Starting upload-deb.sh"

# Uploads the bazel ready tarball for arch, flavour
${SCRIPT_PATH}/upload-deb.sh      \
     "$OUTPUT_DEB_DIR"            \
     "$OUTPUT_BAZEL_ARCHIVE_DIR"  \
     "$OUTPUT_APT_ARCHIVE_DIR"    \
     "$ARCH"                      \
     "$FLAVOUR"                   \
     "$ASTORE_BASE"               \
     "$ASTORE_META_DIR"

echo Done.
exit 1

fi

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
