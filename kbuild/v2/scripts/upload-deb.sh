#!/bin/sh

# This script uploads kernel archives to enkit astore
#
# Inputs
# - a flat directory containing the kernel .debs
# - a directory containing the bazel kernel archives to upload
# - a directory containing the deb archives to upload
# - a space separated list of kernel flavours
# - The astore root where to store kernel artifacts

set -e

LIB_SH="$(dirname $(realpath $0))/lib.sh"
. $LIB_SH

INPUT_DEB_ROOT="$(realpath $1)"
INPUT_BAZEL_ARCHIVE_ROOT="$(realpath $2)"
INPUT_DEB_ARCHIVE_ROOT="$(realpath $3)"
KERNEL_FLAVOURS="$4"
ASTORE_ROOT="$5"

ARCH=amd64

KERNEL_BASE=$(get_kernel_base $INPUT_DEB_ROOT)
if [ -z "$KERNEL_BASE" ] ; then
    echo "ERROR: unable to discover kernel base string"
    exit 1
fi

DEB_VERSION=$(get_deb_version $INPUT_DEB_ROOT)
if [ -z "$DEB_VERSION" ] ; then
    echo "ERROR: unable to discover debian version string"
    exit 1
fi

DEB_TMPDIR=$(mktemp -d)
clean_up()
{
    rm -rf $DEB_TMPDIR
}
trap clean_up EXIT


upload_artifact() {
    local archive="$1"
    local astore_path="$2"
    local arch="$3"
    local public="$4"

    if [ ! -r "$archive" ] ; then
        echo "ERROR: unable to find archive: $archive"
        exit 1
    fi

    # upload archive to astore
    enkit astore upload "${archive}@${astore_path}" -a $arch

    if [ "$public" = "private" ] ; then
        enkit astore public del "$astore_path" > /dev/null 2>&1 || true
    else
        # make all versions public
        enkit astore public add "$astore_path" -a $arch --all > /dev/null 2>&1 || true
    fi

    echo "Upload sha256sum:"
    sha256sum "$archive"

}

upload_bazel_archive() {
    local flavour=$1
    local kernel_version="${KERNEL_BASE}-${flavour}"
    local archive="${INPUT_BAZEL_ARCHIVE_ROOT}/bazel-${kernel_version}.tar.gz"
    local astore_path="${ASTORE_ROOT}/${flavour}/build-headers.tar.gz"

    # these need to be made public for bazel
    upload_artifact "$archive" "$astore_path" "$ARCH" "public"
}

upload_deb_archive() {
    local flavour=$1
    local kernel_version="${KERNEL_BASE}-${flavour}"
    local archive="${INPUT_DEB_ARCHIVE_ROOT}/deb-${kernel_version}.tar.gz"
    local astore_path="${ASTORE_ROOT}/${flavour}/deb-artifacts.tar.gz"

    upload_artifact "$archive" "$astore_path" "$ARCH" "private"
}

upload_kernel_image() {
    local flavour=$1
    local arch=$2
    local kernel_version="${KERNEL_BASE}-${flavour}"
    local kernel_deb="${INPUT_DEB_ROOT}/linux-image-${kernel_version}_${DEB_VERSION}_${ARCH}.deb"
    local tmpdir=$(mktemp -d -p "$DEB_TMPDIR")
    local vmlinuz="${tmpdir}/boot/vmlinuz-${KERNEL_BASE}-${flavour}"
    local astore_path="${ASTORE_ROOT}/${flavour}/vmlinuz"

    if [ ! -r "$kernel_deb" ] ; then
        echo "ERROR: Unable to find kernel .deb package: $kernel_deb"
        exit 1
    fi
    dpkg-deb -x "$kernel_deb" "$tmpdir"

    if [ ! -r "$vmlinuz" ] ; then
        echo "ERROR: Unable to find kernel vmlinuz in deb package: $vmlinuz"
        exit 1
    fi

    upload_artifact "$vmlinuz" "$astore_path" "$ARCH" "public"
}

for f in $KERNEL_FLAVOURS ; do
    upload_bazel_archive $f
    upload_deb_archive $f
    upload_kernel_image $f
done
