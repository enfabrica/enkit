#!/bin/sh

# This script initializes the build area and clones the kernel repo
#
# Inputs:
# - a build directory
# - a kernel branch, e.g. enf/impish-19.19

set -e

KERNEL_SRC_DIR="$(realpath $1)"
KERNEL_REPO="$2"
KERNEL_BRANCH="$3"
KERNEL_VERSION="$4"

make_version() {
    cd "$KERNEL_SRC_DIR"

    # Construct a dynamic local kernel version string based on the current
    # unix epoch and the HEAD git commit hash.

    # Current version of the local version scheme.
    VERSION_SCHEME_VERSION="1"

    timestamp=$(date "+%s")
    githash=$(git rev-parse --short=12 --verify HEAD)

    version="-${VERSION_SCHEME_VERSION}-${timestamp}-g${githash}"
    echo "$version" > "$KERNEL_VERSION"
}

if [ -z "$KERNEL_SRC_DIR" ] ; then
    echo "ERROR: kernel src build directory is not defined"
    exit 1
fi

if [ -z "$KERNEL_REPO" ] ; then
    echo "ERROR: kernel repo not specified"
    exit 1
fi

if [ -z "$KERNEL_BRANCH" ] ; then
    echo "ERROR: kernel branch not specified"
    exit 1
fi

# clean kernel src build dir
if [ "$RT_CLEAN_BUILD" = "yes" ] ; then
    rm -rf "$KERNEL_SRC_DIR" "$KERNEL_VERSION"
fi

if [ -d "$KERNEL_SRC_DIR" -a -r "$KERNEL_VERSION" ] ; then
    # skip cloning the kernel repo.
    exit 0
fi

mkdir -p "$KERNEL_SRC_DIR"

# Shallow clone the kernel tree and branch
git clone --depth 1 --branch "$KERNEL_BRANCH" "$KERNEL_REPO" "$KERNEL_SRC_DIR"

make_version
