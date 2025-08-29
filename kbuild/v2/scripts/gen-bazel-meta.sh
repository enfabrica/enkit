#!/bin/sh

# This script generates kernel version variables for bazel
#
# Inputs
# - a flat directory containing the kernel .debs
# - a space separated list of kernel flavours
# - The astore root where artifacts are stored
# - a directory to store astore meta data files
# - kernel label for creating bazel variable names

set -ex

LIB_SH="$(dirname $(realpath $0))/lib.sh"
. $LIB_SH

OUTPUT_DEB_ROOT="$(realpath $1)"
ARCH="$2"
FLAVOUR="$3"
ASTORE_ROOT="$4"
ASTORE_META_DIR="$5"
KERNEL_LABEL="$6"

bazel_kernel_file="${ASTORE_META_DIR}/kernel-${ARCH}-${FLAVOUR}.version.bzl"
rm -f "$bazel_kernel_file"
touch "$bazel_kernel_file"

# This script only handles one flavour at a time now.
KERNEL_FLAVOURS="$FLAVOUR"

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
    local flavour="$3"
    local astore_file="$4"
    # Note the leading "/", which is different from how the files are
    # uploaded.
    local astore_path="/${ASTORE_ROOT}/${flavour}/${astore_file}"
    local astore_meta="${ASTORE_META_DIR}/${astore_file}-${kernel_version}.json"
    if [ ! -r "$astore_meta" ] ; then
        echo "ERROR: Unable to read astore meta file: $astore_meta"
        exit 1
    fi

# /builder/home/scratch-arm64-generic/kernel-builder/astore-meta/
# vmlinuz-modules.tar.gz-6.3.12-1-1-1756418585-gb1b37559cc40-generic.json

    # Upcase arch and flavour
    local arch="$(echo -n $ARCH | tr [:lower:] [:upper:])"
    local FLAVOUR="$(echo -n $flavour | tr [:lower:] [:upper:])"

    local sha256=$(get_sha256 "$astore_meta")
    local uid=$(get_uid "$astore_meta")

    cat <<EOF >> "$bazel_kernel_file"
${artifact}_${KERNEL_LABEL}_${arch}_${FLAVOUR} = {
    "package":     "enf-${kernel_version}",
    "arch":        "$ARCH",
    "sha256":      "$sha256",
    "astore_path": "$astore_path",
    "astore_uid":  "$uid",
}

EOF

}

gen_deb_flavours() {
    for f in $KERNEL_FLAVOURS ; do
        local kernel_version="$(deb_get_kernel_version $OUTPUT_DEB_ROOT $f)"

        ## build-headers.tar.gz
        local astore_file="build-headers.tar.gz"
        gen_artifact_desc "KERNEL_TREE" $kernel_version $f $astore_file

        if [ "$ARCH" = "amd64" ] && [ "$f" = "generic" ] ; then

        ## vmlinuz-modules.tar.gz
        local astore_file="vmlinuz-modules.tar.gz"
        gen_artifact_desc "KERNEL_BIN" $kernel_version $f $astore_file

        else
            echo "Skipping KERNEL_BIN for $ARCH-$f"

        fi

        ## deb-artifacts.tar.gz
        local astore_file="deb-artifacts.tar.gz"
        gen_artifact_desc "KERNEL_DEB" $kernel_version $f $astore_file

    done
}

gen_deb_flavours
