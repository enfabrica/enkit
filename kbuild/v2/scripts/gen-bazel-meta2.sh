#!/bin/sh

# This script generates kernel version variables for bazel
#
# Inputs
# - The astore root where artifacts are stored
# - a directory to store astore meta data files
# - kernel label for creating bazel variable names

set -e

LIB_SH="$(dirname $(realpath $0))/lib.sh"
. $LIB_SH

KERNEL_BUILD_DIR="$(realpath $1)"
KRELEASE_DIR="$(realpath $2)"
ARCH="$3"
FLAVOUR="$4"
ASTORE_ROOT="$5"
ASTORE_META_DIR="$6"
KERNEL_LABEL="$7"

kernel_version="$(cat ${KERNEL_BUILD_DIR}/install/build/enf-kernel-version.txt)"

bazel_kernel_file="${ASTORE_META_DIR}/kernel-${ARCH}-${FLAVOUR}.version.bzl"
rm -f "$bazel_kernel_file"
touch "$bazel_kernel_file"

get_sha256() {
    local astore_meta="$1"
    local sha256=$(cat "$astore_meta" | jq '.sha256' | sed -e 's/"//g')
    if [ -z "$sha256" ] ; then
        echo "ERROR: Unable to find sha256 in astore meta: $astore_meta"
        exit 1
    fi
    echo -n "$sha256"
}

get_uid() {
    local astore_meta="$1"
    local uid=$(cat "$astore_meta" | jq '.uid' | sed -e 's/"//g')
    if [ -z "$sha256" ] ; then
        echo "ERROR: Unable to find uid in astore meta: $astore_meta"
        exit 1
    fi
    echo -n "$uid"
}

gen_artifact_desc() {
    local artifact="$1"
    local kernel_version="$2"
    local astore_file="$3"
    local astore_path="$4"
    local kernel_label="$5"

    local astore_meta="${ASTORE_META_DIR}/${astore_file}.json"
    if [ ! -r "$astore_meta" ] ; then
        echo "ERROR: Unable to read astore meta file: $astore_meta"
        exit 1
    fi

    local sha256=$(get_sha256 "$astore_meta")
    local uid=$(get_uid "$astore_meta")

    cat <<EOF >> "$bazel_kernel_file"
${artifact}_${kernel_label} = {
    "package":     "enf-${kernel_version}",
    "arch":        "$ARCH",
    "sha256":      "$sha256",
    "astore_path": "$astore_path",
    "astore_uid":  "$uid",
}

EOF

}

astore_file="kernel-tree-image-${ARCH}-${FLAVOUR}.tar.gz"
# Note the leading "/", which is different from how the files are
# uploaded.
astore_path="/${ASTORE_ROOT}/${ARCH}/${FLAVOUR}/${astore_file}"

# translate ARCH to bazel speak if necessary
if [ "$ARCH" = "arm64" ] ; then
    ARCH="aarch64"
fi

upper_arch="$(echo $ARCH | tr '[:lower:]' '[:upper:]')"
upper_flavour="$(echo $FLAVOUR | tr '[:lower:]' '[:upper:]')"
kernel_label="${KERNEL_LABEL}_${upper_arch}_${upper_flavour}"

gen_artifact_desc "KERNEL_TREE_IMAGE" $kernel_version $astore_file $astore_path $kernel_label
