#!/bin/sh

# This script uploads kernel build tarball to enkit astore
#
# Inputs
# - kernel version string
# - kernel build tarball
# - The astore root where to store kernel artifacts
# - a directory to store astore meta data files

set -e

LIB_SH="$(dirname $(realpath $0))/lib.sh"
. $LIB_SH

KERNEL_BUILD_DIR="$(realpath $1)"
KRELEASE_DIR="$(realpath $2)"
ARCH="$3"
FLAVOUR="$4"
ASTORE_ROOT="$5"
ASTORE_META_DIR="$6"

kernel_version="$(cat ${KERNEL_BUILD_DIR}/install/build/enf-kernel-version.txt)"

kernel_release_tarball="${KRELEASE_DIR}/kernel-tree-image.tar.gz"
if [ ! -r "$kernel_release_tarball" ] ; then
    echo "ERROR: unable to find kernel release tarball: $kernel_release_tarball"
    exit 1
fi

if [ ! -d "$ASTORE_META_DIR" ] ; then
    echo "ERROR: unable to find astore meta-data directory: $ASTORE_META_DIR"
    exit 1
fi

astore_file="kernel-tree-image-${ARCH}-${FLAVOUR}.tar.gz"
astore_path="${ASTORE_ROOT}/${ARCH}/${FLAVOUR}/${astore_file}"
astore_meta="${ASTORE_META_DIR}/${astore_file}.json"

upload_artifact "$kernel_release_tarball" "$astore_path" "$ARCH" "$kernel_version" "$astore_meta"
