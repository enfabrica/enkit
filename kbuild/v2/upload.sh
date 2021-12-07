#!/bin/sh

# This script uploads kernel archives to enkit astore
#
# Inputs
# - a flat directory containing the kernel .debs
# - a directory containing the kernel archives to upload
# - a space separated list of kernel flavours
# - The astore root where to store kernel artifacts

set -e

LIB_SH="$(dirname $(realpath $0))/lib.sh"
. $LIB_SH

INPUT_DEB_ROOT="$(realpath $1)"
INPUT_ARCHIVE_ROOT="$(realpath $2)"
KERNEL_FLAVOURS="$3"
ASTORE_ROOT="$4"

ARCH=amd64

KERNEL_BASE=$(get_kernel_base $INPUT_DEB_ROOT)
if [ -z "$KERNEL_BASE" ] ; then
    echo "ERROR: unable to discover kernel base string"
    exit 1
fi

upload_archive() {
    local flavour=$1
    local kernel_version="${KERNEL_BASE}-${flavour}"
    local archive="${INPUT_ARCHIVE_ROOT}/${kernel_version}.tar.gz"

    if [ ! -r "$archive" ] ; then
        echo "ERROR: unable to find archive: $archive"
        exit 1
    fi

    # upload tarball to astore
    local astore_path="${ASTORE_ROOT}/${flavour}/kernel-artifacts.tar.gz"
    enkit astore upload "${archive}@${astore_path}" -a $ARCH

    # make it public
    enkit astore public del "$astore_path" > /dev/null 2>&1 || true
    enkit astore public add "$astore_path" -a $ARCH

    echo "Upload sha256sum:"
    sha256sum "$archive"
}

for f in $KERNEL_FLAVOURS ; do
    upload_archive $f
done
