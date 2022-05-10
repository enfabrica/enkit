#!/bin/sh

# This script uploads kernel archives to enkit astore
#
# Inputs
# - the kernel build directory
# - the UML bazel archive output directory
# - The astore root where to store kernel artifacts

set -e

OUTPUT_UML_DIR="$(realpath $1)"
OUTPUT_UML_BAZEL_ARCHIVE_DIR="$(realpath $2)"
ASTORE_ROOT="$3"

uml_image_src="${OUTPUT_UML_DIR}/linux"
if [ ! -r "$uml_image_src" ] ; then
    echo "ERROR: unable to read UML kernel image: $uml_image"
    exit 1
fi
uml_image_astore_dest="${ASTORE_ROOT}/test/enf-uml-img"

archive_src="${OUTPUT_UML_BAZEL_ARCHIVE_DIR}/build-headers.tar.gz"
if [ ! -r "$archive_src" ] ; then
    echo "ERROR: unable to read UML kernel archive: $archive_src"
    exit 1
fi
archive_astore_dest="${ASTORE_ROOT}/test/build-headers.tar.gz"

upload_artifact() {
    local archive="$1"
    local astore_path="$2"
    local arch="$3"

    if [ ! -r "$archive" ] ; then
        echo "ERROR: unable to find archive: $archive"
        exit 1
    fi

    # upload archive to astore
    enkit astore upload "${archive}@${astore_path}" -a $arch

    # make all versions public
    enkit astore public add "$astore_path" -a $arch --all > /dev/null 2>&1 || true

    echo "Upload sha256sum:"
    sha256sum "$archive"

}

upload_artifact "$uml_image_src" "$uml_image_astore_dest" "um"
upload_artifact "$archive_src"   "$archive_astore_dest"   "um"
