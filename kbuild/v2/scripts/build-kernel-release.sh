#!/bin/sh

# This script builds a kernel release tarball
#
# Inputs:
# - kernel source directory
# - kernel build directory
# - the kernel version file
# - CPU architecture
# - kernel flavour
# - an output directory to place the generated tarball

set -e

SCRIPT_PATH="$(dirname $(realpath $0))"
. "${SCRIPT_PATH}/lib.sh"

KERNEL_SRC_DIR="$(realpath $1)"
BUILD_DIR="$(realpath -m $2)"
KERNEL_SUFFIX="$(realpath $3)"
ARCH="$4"
FLAVOUR="$5"
OUTPUT_KRELEASE_DIR="$(realpath -m $6)"

if [ ! -d "$KERNEL_SRC_DIR" ] ; then
    echo "ERROR: kernel source directory does not exist: $KERNEL_SRC_DIR"
    exit 1
fi

if [ ! -r "$KERNEL_SUFFIX" ] ; then
    echo "ERROR: unable to read kernel version suffix file"
    exit 1
fi

kernel_version_suffix=$(cat "$KERNEL_SUFFIX")

# clean output kernel build and release dirs
if [ "$RT_CLEAN_BUILD" = "yes" ] ; then
    rm -rf "$BUILD_DIR"
    rm -rf "$OUTPUT_KRELEASE_DIR"
fi

if [ -d "$OUTPUT_KRELEASE_DIR" ] ; then
    # skip building the kernel release tarball
    exit 0
fi

mkdir -p "$BUILD_DIR" "$OUTPUT_KRELEASE_DIR"

ksrc_dir="${BUILD_DIR}/source"
rsync -a "${KERNEL_SRC_DIR}/" "$ksrc_dir"

# The following script should be a script that kernel devs can call
# directly for setting up a development kernel area for bazel...

cd "$ksrc_dir"
${SCRIPT_PATH}/build-kernel-tree.sh -q -c -v "$kernel_version_suffix" -a "$ARCH" -f "$FLAVOUR"

# above script creates sibling dirs of $ksrc_dir named "boot" and
# "install" and an installer script.

kernel_version="$(cat ${BUILD_DIR}/install/build/enf-kernel-version.txt)"

# remove a bunch of unneeded stuff from build directory
PATTERNS=".*.cmd *.a *.o *.d *.ko *.order *.mod *.mod.c *.mod.o *.log"
for p in $PATTERNS ; do
    find "${BUILD_DIR}/install" -name $p -type f -exec rm -f {} +
done

# TODO: remove even more stuff from the "source" and "install" directory

# now tar up the results
# - kernel source
# - boot dir
# - install dir
# - installer script
kernel_release_tarball="${OUTPUT_KRELEASE_DIR}/kernel-tree-image.tar.gz"
tar czf "$kernel_release_tarball" -C "$BUILD_DIR" source install boot "install-${kernel_version}.sh"
