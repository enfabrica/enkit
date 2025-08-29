#!/bin/sh

# This script builds the .debs for an Enfabrica kernel
#
# Inputs:
# - kernel source direcotry
# - the kernel version file
# - CPU architecture
# - kernel flavour
# - a build directory
# - an output directory to place the generated .debs

set -e

KERNEL_SRC_DIR="$(realpath $1)"
KERNEL_VERSION="$(realpath $2)"
ARCH="$3"
FLAVOUR="$4"
BUILD_DIR="$5"
OUTPUT_DEB_DIR="$6"

if [ "$FLAVOUR" = "generic" ] ; then
    : # continue
else
    echo "Not implemented: FLAVOUR=$FLAVOUR"
    exit 1
fi

if [ ! -d "$KERNEL_SRC_DIR" ] ; then
    echo "ERROR: kernel source directory does not exist: $KERNEL_SRC_DIR"
    exit 1
fi

if [ ! -r "$KERNEL_VERSION" ] ; then
    echo "ERROR: unable to read kernel version file"
    exit 1
fi
kernel_version=$(cat "$KERNEL_VERSION")

# clean output .deb dir
if [ "$RT_CLEAN_BUILD" = "yes" ] ; then
    rm -rf "$BUILD_DIR"
    rm -rf "$OUTPUT_DEB_DIR"
fi

if [ -d "$OUTPUT_DEB_DIR" ] ; then
    # skip building the kernel .debs
    exit 0
fi

mkdir -p "$BUILD_DIR" "$OUTPUT_DEB_DIR"

ksrc_dir="${BUILD_DIR}/source"
rsync -a "${KERNEL_SRC_DIR}/" "$ksrc_dir"

cd "$ksrc_dir"

# If the git tree has any uncommitted modifications, mark the
# version as dirty.
if [ -n "$(git status --porcelain)" ] ; then
    echo "WARNING: The build tree contains uncommitted changes."
    dirty="-dirty"
else
    dirty=
fi

abi_suffix="${kernel_version}${dirty}"

if [ "$RT_BUILD_CLEAN" = "yes" ] ; then
    fakeroot debian/rules distclean
fi

if [ "$ARCH" = "arm64" ]; then
    # export CROSS_COMPILE=aarch64-linux-gnu-
    # in the top level script
    export CROSS_COMPILE=aarch64-none-linux-gnu-
    export DEB_HOST_ARCH=arm64
    export DEB_BUILD_PROFILES="cross nocheck"
fi

fakeroot debian/rules clean	   abi_suffix="$abi_suffix" arch="$ARCH" flavours="$FLAVOUR"
fakeroot debian/rules binary-debs  abi_suffix="$abi_suffix" arch="$ARCH" flavours="$FLAVOUR"
fakeroot debian/rules binary-indep abi_suffix="$abi_suffix" arch="$ARCH" flavours="$FLAVOUR"

# mv the resulting .debs
mv ../linux-*deb "$OUTPUT_DEB_DIR"
