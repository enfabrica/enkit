#!/bin/sh

# This script uploads kernel archives to enkit astore
#
# Inputs
# - the kernel build directory
# - the UML bazel archive output directory
# - The astore root where to store kernel artifacts

set -e

LIB_SH="$(dirname $(realpath $0))/lib.sh"
. $LIB_SH

OUTPUT_UML_DIR="$(realpath $1)"
OUTPUT_UML_BAZEL_ARCHIVE_DIR="$(realpath $2)"
ASTORE_ROOT="$3"

uml_image_src="${OUTPUT_UML_DIR}/linux"
if [ ! -r "$uml_image_src" ] ; then
    echo "ERROR: unable to read UML kernel image: $uml_image"
    exit 1
fi
uml_image_astore_dest="${ASTORE_ROOT}/test/vmlinuz"

archive_src="${OUTPUT_UML_BAZEL_ARCHIVE_DIR}/build-headers.tar.gz"
if [ ! -r "$archive_src" ] ; then
    echo "ERROR: unable to read UML kernel archive: $archive_src"
    exit 1
fi
archive_astore_dest="${ASTORE_ROOT}/test/build-headers.tar.gz"

modules_tar="${OUTPUT_UML_DIR}/modules.tar.gz"
modules_astore_dest="${ASTORE_ROOT}/test/modules.tar.gz"

kernel_version="$(cat ${OUTPUT_UML_DIR}/include/config/kernel.release)"
tag="kernel=$kernel_version"

upload_artifact "$uml_image_src" "$uml_image_astore_dest" "um" "$tag"
upload_artifact "$archive_src"   "$archive_astore_dest"   "um" "$tag"
upload_artifact "$modules_tar"   "$modules_astore_dest"   "um" "$tag"
