#!/bin/sh

# This script uploads kernel archives to enkit astore
#
# Inputs
# - the kernel build directory
# - the UML bazel archive output directory
# - The astore root where to store kernel artifacts
# - a directory to store astore meta data files

set -e

LIB_SH="$(dirname $(realpath $0))/lib.sh"
. $LIB_SH

OUTPUT_UML_DIR="$(realpath $1)"
OUTPUT_UML_BAZEL_ARCHIVE_DIR="$(realpath $2)"
ASTORE_ROOT="$3"
ASTORE_META_DIR="$4"

uml_image="${OUTPUT_UML_DIR}/linux"
uml_modules_dir="${OUTPUT_UML_DIR}/modules-install"

archive_src="${OUTPUT_UML_BAZEL_ARCHIVE_DIR}/build-headers.tar.gz"

if [ ! -d "$ASTORE_META_DIR" ] ; then
    echo "ERROR: unable to find astore meta-data directory: $ASTORE_META_DIR"
    exit 1
fi

flavour="test"
kernel_version="$(uml_get_kernel_version $OUTPUT_UML_DIR)"
tag="kernel=$kernel_version"

TAR_TMPDIR=$(mktemp -d)
clean_up()
{
    rm -rf $TAR_TMPDIR
}
trap clean_up EXIT

upload_uml_bazel_archive() {
    local flavour=$1
    local kernel_version="$2"
    local tag="$3"
    local archive="$4"
    local astore_file="build-headers.tar.gz"
    local astore_path="${ASTORE_ROOT}/${flavour}/${astore_file}"
    local astore_meta="${ASTORE_META_DIR}/${astore_file}-${kernel_version}.json"

    if [ ! -r "$archive" ] ; then
        echo "ERROR: unable to read UML kernel archive: $archive"
        exit 1
    fi

    upload_artifact "$archive" "$astore_path" "um" "$tag" "$astore_meta"
}

upload_uml_kernel_image_modules() {
    local flavour=$1
    local kernel_version="$2"
    local tag="$3"
    local uml_image="$4"
    local uml_modules_dir="$5"
    local tmpdir=$(mktemp -d -p "$TAR_TMPDIR")
    local vmlinuz="${tmpdir}/boot/vmlinuz-${kernel_version}"
    local tarball="${TAR_TMPDIR}/vmlinuz-modules.tar.gz"
    local astore_file="vmlinuz-modules.tar.gz"
    local astore_path="${ASTORE_ROOT}/${flavour}/${astore_file}"
    local astore_meta="${ASTORE_META_DIR}/${astore_file}-${kernel_version}.json"

    if [ ! -r "$uml_image" ] ; then
        echo "ERROR: unable to read UML kernel image: $uml_image"
        exit 1
    fi

    if [ ! -d "${uml_modules_dir}/lib" ] ; then
        echo "ERROR: unable to read UML kernel module install directory: ${uml_modules_dir}/lib"
        exit 1
    fi

    # copy vmlinuz into tarball directory and make it executable
    mkdir -p "${tmpdir}/boot"
    /bin/cp "$uml_image" "${tmpdir}/boot/vmlinuz-${kernel_version}"
    chmod a+x "${tmpdir}/boot/vmlinuz-${kernel_version}"

    # copy modules into tarball directory
    /bin/cp -r "${uml_modules_dir}/lib" "$tmpdir"

    # Include the /boot and /lib directories
    tar -C "$tmpdir" --owner root --group root --create --gzip --file "$tarball" boot lib

    upload_artifact "$tarball" "$astore_path" "um" "$tag" "$astore_meta"
}

upload_uml_bazel_archive        "$flavour" "$kernel_version" "$tag" "$archive_src"
upload_uml_kernel_image_modules "$flavour" "$kernel_version" "$tag" "$uml_image" "$uml_modules_dir"
