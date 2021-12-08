#!/bin/sh

# This script generates a bazel tarball for building kernel modules
#
# Inputs:
# - a flat directory containing the kernel .debs
# - a space separated list of kernel flavours
# - an output directory to place the generated bazel archive

set -e

LIB_SH="$(dirname $(realpath $0))/lib.sh"
. $LIB_SH

INPUT_DEB_ROOT="$(realpath $1)"
KERNEL_FLAVOURS="$2"
OUTPUT_ARCHIVE_ROOT="$(realpath $3)"

INSTALL_TEMPLATE="$(dirname $(realpath $0))/template/install-bazel.sh"
if [ ! -r "$INSTALL_TEMPLATE" ] ; then
    echo "ERROR: unable to find bazel install script template: $INSTALL_TEMPLATE"
    exit 1
fi

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

rm -rf "$OUTPUT_ARCHIVE_ROOT"
mkdir -p "$OUTPUT_ARCHIVE_ROOT"

make_bazel_archive() {
    local flavour=$1

    local tmp_dir="${OUTPUT_ARCHIVE_ROOT}/tmp"
    mkdir -p "$tmp_dir"

    local kernel_version="${KERNEL_BASE}-${flavour}"
    local archive="${OUTPUT_ARCHIVE_ROOT}/bazel-${kernel_version}.tar.gz"

    local install_script="${tmp_dir}/install-${kernel_version}.sh"

    # sed some vars from a template and make the install script ...
    rm -f "$install_script"
    cp  "$INSTALL_TEMPLATE" "$install_script"

    # substitute some vars into the template
    local KERNEL_FLAVOUR="$flavour"
    for var in KERNEL_FLAVOUR KERNEL_BASE DEB_VERSION KERNEL_VERSION ARCH; do
        sed -i -e "s/%%${var}%%/$(eval echo -n \$$var)/" $install_script
    done
    chmod 755 "$install_script"

    # put the common and arch specific linux-header debs in the archive
    local common_linux_headers_deb="${INPUT_DEB_ROOT}/linux-headers-${KERNEL_BASE}_${DEB_VERSION}_all.deb"
    if [ ! -r "$common_linux_headers_deb" ] ; then
        echo "ERROR: unable to find common header .deb: $common_linux_headers_deb"
        exit 1
    fi
    cp "$common_linux_headers_deb" "$tmp_dir"

    local arch_linux_headers_deb="${INPUT_DEB_ROOT}/linux-headers-${KERNEL_BASE}-${KERNEL_FLAVOUR}_${DEB_VERSION}_${ARCH}.deb"
    if [ ! -r "$arch_linux_headers_deb" ] ; then
        echo "ERROR: unable to find arch header .deb: $arch_linux_headers_deb"
        exit 1
    fi
    cp "$arch_linux_headers_deb" "$tmp_dir"

    # create tarball
    echo -n "${flavour}: Generating bazel build archive file... "
    tar -C "$tmp_dir" --create --gzip --file "$archive" .
    rm -rf "$tmp_dir"

    echo "Done."
}

for f in $KERNEL_FLAVOURS ; do
    make_bazel_archive $f
done
