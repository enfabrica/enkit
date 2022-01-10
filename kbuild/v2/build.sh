#!/bin/sh

# This script builds the .debs for an Enfabrica kernel
#
# Inputs:
# - a build directory
# - a kernel branch, e.g. enf/impish-19.19
# - a space delimited list of kernel flavours, e.g. "minimal generic"
# - an output directory to place the generated .debs

set -e

BUILD_DIR="$(realpath $1)"
KERNEL_BRANCH="$2"
KERNEL_FLAVOURS="$3"
OUTPUT_DEB_DIR="$(realpath $4)"

if [ -z "$BUILD_DIR" ] ; then
    echo "ERROR: build directory is not defined"
    exit 1
fi

if [ -z "$KERNEL_BRANCH" ] ; then
    echo "ERROR: kernel branch not specified"
    exit 1
fi

if [ -z "$KERNEL_FLAVOURS" ] ; then
    echo "ERROR: kernel flavours not specified: valid values 'minimal generic'"
    exit 1
fi

KERNEL_DIR="${BUILD_DIR}/enf-linux"

# clean build dir
mkdir -p $BUILD_DIR
rm -f "${BUILD_DIR}/linux-"*deb || true

# clean output .deb dir
rm -rf "$OUTPUT_DEB_DIR"
mkdir -p "$OUTPUT_DEB_DIR"

# Get latest kernel tree
if [ ! -d "$KERNEL_DIR" ] ; then
    git clone git@github.com:enfabrica/linux.git "$KERNEL_DIR"
else
    cd $KERNEL_DIR
    git fetch origin
    cd - > /dev/null 2>&1
fi

# Check out latest branch
cd $KERNEL_DIR
git checkout main
git branch -D $KERNEL_BRANCH > /dev/null 2>&1 || true
git checkout $KERNEL_BRANCH
git clean -dfx

# build all the flavours
./enfabrica/build-kernel.sh -b auto -f "$KERNEL_FLAVOURS"

# copy out the resulting .debs
cp -av ../linux-*deb "$OUTPUT_DEB_DIR"
