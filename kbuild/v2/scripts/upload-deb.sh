#!/bin/sh

# This script uploads kernel archives to enkit astore
#
# Inputs
# - a flat directory containing the kernel .debs
# - a directory containing the bazel kernel archives to upload
# - a directory containing the deb archives to upload
# - a space separated list of kernel flavours
# - The astore root where to store kernel artifacts
# - a directory to store astore meta data files

set -ex

LIB_SH="$(dirname $(realpath $0))/lib.sh"
. $LIB_SH

INPUT_DEB_ROOT="$(realpath $1)"
INPUT_BAZEL_ARCHIVE_ROOT="$(realpath $2)"
INPUT_DEB_ARCHIVE_ROOT="$(realpath $3)"
ARCH="$4"
FLAVOUR="$5"
ASTORE_ROOT="$6"
ASTORE_META_DIR="$7"

# This script only handles one flavour at a time now.
KERNEL_FLAVOURS="$FLAVOUR"

DEB_VERSION=$(get_deb_version $INPUT_DEB_ROOT)
if [ -z "$DEB_VERSION" ] ; then
    echo "ERROR: unable to discover debian version string"
    exit 1
fi

if [ ! -d "$ASTORE_META_DIR" ] ; then
    echo "ERROR: unable to find astore meta-data directory: $ASTORE_META_DIR"
    exit 1
fi

DEB_TMPDIR=$(mktemp -d)
clean_up()
{
    rm -rf $DEB_TMPDIR
}
trap clean_up EXIT

kernel_tag() {
    local flavour=$1
    local tag="kernel=$(deb_get_kernel_version $INPUT_DEB_ROOT $flavour)"

    echo -n "$tag"
}

upload_bazel_archive() {
    local flavour=$1
    local kernel_version="$2"
    local archive="${INPUT_BAZEL_ARCHIVE_ROOT}/bazel-${kernel_version}.tar.gz"
    local astore_file="build-headers.tar.gz"
    local astore_path="${ASTORE_ROOT}/${flavour}/${astore_file}"
    local tag="$(kernel_tag $flavour)"
    local astore_meta="${ASTORE_META_DIR}/${astore_file}-${kernel_version}.json"

    upload_artifact "$archive" "$astore_path" "$ARCH" "$tag" "$astore_meta"
}

upload_deb_archive() {
    local flavour=$1
    local kernel_version="$2"
    local archive="${INPUT_DEB_ARCHIVE_ROOT}/deb-${kernel_version}.tar.gz"
    local astore_file="deb-artifacts.tar.gz"
    local astore_path="${ASTORE_ROOT}/${flavour}/${astore_file}"
    local tag="$(kernel_tag $flavour)"
    local astore_meta="${ASTORE_META_DIR}/${astore_file}-${kernel_version}.json"

    upload_artifact "$archive" "$astore_path" "$ARCH" "$tag" "$astore_meta"
}

upload_kernel_image_modules() {
    local flavour=$1
    local kernel_version="$2"
    local kernel_deb="${INPUT_DEB_ROOT}/linux-image-${kernel_version}_${DEB_VERSION}_${ARCH}.deb"
    local modules_deb="${INPUT_DEB_ROOT}/linux-modules-${kernel_version}_${DEB_VERSION}_${ARCH}.deb"
    local tmpdir=$(mktemp -d -p "$DEB_TMPDIR")
    local vmlinuz="${tmpdir}/boot/vmlinuz-${kernel_version}"
    local tarball="${DEB_TMPDIR}/vmlinuz-modules.tar.gz"
    local astore_file="vmlinuz-modules.tar.gz"
    local astore_path="${ASTORE_ROOT}/${flavour}/${astore_file}"
    local tag="$(kernel_tag $flavour)"
    local astore_meta="${ASTORE_META_DIR}/${astore_file}-${kernel_version}.json"

    if [ ! -r "$kernel_deb" ] ; then
        echo "ERROR: Unable to find kernel .deb package: $kernel_deb"
        exit 1
    fi

    if [ ! -r "$modules_deb" ] ; then
        echo "ERROR: Unable to find kernel modules .deb package: $modules_deb"
        exit 1
    fi

    dpkg-deb -x "$kernel_deb" "$tmpdir"
    if [ ! -r "$vmlinuz" ] ; then
        echo "ERROR: Unable to find kernel vmlinuz in deb package: $vmlinuz"
        exit 1
    fi

    dpkg-deb -x "$modules_deb" "$tmpdir"

    # Include the /boot and /lib directories
    tar -C "$tmpdir" --owner root --group root --create --gzip --file "$tarball" boot lib

    upload_artifact "$tarball" "$astore_path" "$ARCH" "$tag" "$astore_meta"
}

for f in $KERNEL_FLAVOURS ; do
    kernel_version="$(deb_get_kernel_version $INPUT_DEB_ROOT $f)"

    upload_bazel_archive        $f $kernel_version
    upload_deb_archive          $f $kernel_version
    upload_kernel_image_modules $f $kernel_version
done
