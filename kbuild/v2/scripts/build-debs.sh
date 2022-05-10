#!/bin/sh

# This script builds the .debs for an Enfabrica kernel
#
# Inputs:
# - a build directory
# - the kernel version file
# - a space delimited list of kernel flavours, e.g. "minimal generic"
# - an output directory to place the generated .debs

set -e

KERNEL_DIR="$(realpath $1)"
KERNEL_VERSION="$(realpath $2)"
KERNEL_FLAVOURS="$3"
OUTPUT_DEB_DIR="$(realpath $4)"

if [ -z "$KERNEL_DIR" ] ; then
    echo "ERROR: kernel build directory is not defined"
    exit 1
fi

if [ ! -r "$KERNEL_VERSION" ] ; then
    echo "ERROR: unable to read kernel version file"
    exit 1
fi

if [ -z "$KERNEL_FLAVOURS" ] ; then
    echo "ERROR: kernel flavours not specified: valid values 'minimal generic'"
    exit 1
fi
kernel_version=$(cat "$KERNEL_VERSION")

# clean output .deb dir
if [ "$RT_CLEAN_BUILD" = "yes" ] ; then
    rm -rf "$OUTPUT_DEB_DIR"
fi

if [ -d "$OUTPUT_DEB_DIR" ] ; then
    # skip building the kernel .debs
    exit 0
fi

mkdir -p "$OUTPUT_DEB_DIR"

# build all the flavours
cd "$KERNEL_DIR"
./enfabrica/build-kernel.sh -b "local=$kernel_version" -f "$KERNEL_FLAVOURS"

# copy out the resulting .debs
cp -av ../linux-*deb "$OUTPUT_DEB_DIR"
