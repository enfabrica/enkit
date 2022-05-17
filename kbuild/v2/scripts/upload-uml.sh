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

uml_image_src="${OUTPUT_UML_DIR}/linux"
if [ ! -r "$uml_image_src" ] ; then
    echo "ERROR: unable to read UML kernel image: $uml_image"
    exit 1
fi

archive_src="${OUTPUT_UML_BAZEL_ARCHIVE_DIR}/build-headers.tar.gz"
if [ ! -r "$archive_src" ] ; then
    echo "ERROR: unable to read UML kernel archive: $archive_src"
    exit 1
fi

if [ ! -d "$ASTORE_META_DIR" ] ; then
    echo "ERROR: unable to find astore meta-data directory: $ASTORE_META_DIR"
    exit 1
fi

kernel_version="$(uml_get_kernel_version $OUTPUT_UML_DIR)"
tag="kernel=$kernel_version"

uml_image_astore_file="vmlinuz"
uml_image_astore_path="${ASTORE_ROOT}/test/${uml_image_astore_file}"
uml_image_astore_meta="${ASTORE_META_DIR}/${uml_image_astore_file}-${kernel_version}.json"

archive_astore_file="build-headers.tar.gz"
archive_astore_path="${ASTORE_ROOT}/test/$archive_astore_file"
archive_astore_meta="${ASTORE_META_DIR}/${archive_astore_file}-${kernel_version}.json"

upload_artifact "$uml_image_src" "$uml_image_astore_path" "um" "$tag" "$uml_image_astore_meta"
upload_artifact "$archive_src"   "$archive_astore_path"   "um" "$tag" "$archive_astore_meta"
