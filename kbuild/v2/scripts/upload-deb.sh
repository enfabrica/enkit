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


kernel_version() {
    local flavour=$1
    local kernel_version="${KERNEL_BASE}-${flavour}"

    echo -n "$kernel_version"
}

kernel_tag() {
    local flavour=$1
    local tag="kernel=$(kernel_version $flavour)"

    echo -n "$tag"
}

upload_bazel_archive() {
    local flavour=$1
    local kernel_version="$(kernel_version $flavour)"
    local archive="${INPUT_BAZEL_ARCHIVE_ROOT}/bazel-${kernel_version}.tar.gz"
    local astore_path="${ASTORE_ROOT}/${flavour}/build-headers.tar.gz"
    local tag="$(kernel_tag $flavour)"

    upload_artifact "$archive" "$astore_path" "$ARCH" "$tag"
}

upload_deb_archive() {
    local flavour=$1
    local kernel_version="$(kernel_version $flavour)"
    local archive="${INPUT_DEB_ARCHIVE_ROOT}/deb-${kernel_version}.tar.gz"
    local astore_path="${ASTORE_ROOT}/${flavour}/deb-artifacts.tar.gz"
    local tag="$(kernel_tag $flavour)"

    upload_artifact "$archive" "$astore_path" "$ARCH" "$tag"
}

upload_kernel_image() {
    local flavour=$1
    local kernel_version="$(kernel_version $flavour)"
    local kernel_deb="${INPUT_DEB_ROOT}/linux-image-${kernel_version}_${DEB_VERSION}_${ARCH}.deb"
    local tmpdir=$(mktemp -d -p "$DEB_TMPDIR")
    local vmlinuz="${tmpdir}/boot/vmlinuz-${KERNEL_BASE}-${flavour}"
    local astore_path="${ASTORE_ROOT}/${flavour}/vmlinuz"
    local tag="$(kernel_tag $flavour)"

    if [ ! -r "$kernel_deb" ] ; then
        echo "ERROR: Unable to find kernel .deb package: $kernel_deb"
        exit 1
    fi
    dpkg-deb -x "$kernel_deb" "$tmpdir"

    if [ ! -r "$vmlinuz" ] ; then
        echo "ERROR: Unable to find kernel vmlinuz in deb package: $vmlinuz"
        exit 1
    fi

    upload_artifact "$vmlinuz" "$astore_path" "$ARCH" "$tag"
}

upload_kernel_modules() {
    local flavour=$1
    local kernel_version="$(kernel_version $flavour)"
    local modules_deb="${INPUT_DEB_ROOT}/linux-modules-${kernel_version}_${DEB_VERSION}_${ARCH}.deb"
    local tmpdir=$(mktemp -d -p "$DEB_TMPDIR")
    local modules_tar="${DEB_TMPDIR}/modules.tar.gz"
    local astore_path="${ASTORE_ROOT}/${flavour}/modules.tar.gz"
    local tag="$(kernel_tag $flavour)"

    if [ ! -r "$modules_deb" ] ; then
        echo "ERROR: Unable to find kernel modules .deb package: $modules_deb"
        exit 1
    fi
    dpkg-deb -x "$modules_deb" "$tmpdir"

    tar -C "$tmpdir" --owner root --group root --create --gzip --file "$modules_tar" .

    upload_artifact "$modules_tar" "$astore_path" "$ARCH" "$tag"
}

for f in $KERNEL_FLAVOURS ; do
    upload_bazel_archive $f
    upload_deb_archive $f
    upload_kernel_image $f
    upload_kernel_modules $f
done
