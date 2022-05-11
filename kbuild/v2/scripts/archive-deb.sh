#!/bin/sh

# This script generates an astore tarball for each Debian repository
#
# Inputs:
# - a flat directory containing the kernel .debs
# - a Debian APT repo
# - a space separated list of kernel flavours
# - an output directory to place the generated APT repo

set -e

LIB_SH="$(dirname $(realpath $0))/lib.sh"
. $LIB_SH

INPUT_DEB_ROOT="$(realpath $1)"
INPUT_REPO_ROOT="$(realpath $2)"
KERNEL_FLAVOURS="$3"
OUTPUT_ARCHIVE_ROOT="$(realpath $4)"

INSTALL_TEMPLATE="$(dirname $(realpath $0))/../template/install-deb.sh"
if [ ! -r "$INSTALL_TEMPLATE" ] ; then
    echo "ERROR: unable to find install script template: $INSTALL_TEMPLATE"
    exit 1
fi

ARCH=amd64
DIST=focal
COMP=main

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

make_archive() {
    local flavour=$1

    local flavour_repo="${INPUT_REPO_ROOT}/${flavour}"
    local kernel_version="${KERNEL_BASE}-${flavour}"
    local archive="${OUTPUT_ARCHIVE_ROOT}/deb-${kernel_version}.tar.gz"

    local install_script="${flavour_repo}/install-${kernel_version}.sh"

    # sed some vars from a template and make the install script ...
    rm -f "$install_script"
    cp  "$INSTALL_TEMPLATE" "$install_script"

    # substitute some vars into the template
    local KERNEL_FLAVOUR="$flavour"
    for var in KERNEL_FLAVOUR KERNEL_BASE DEB_VERSION KERNEL_VERSION ARCH DIST COMP; do
        sed -i -e "s/%%${var}%%/$(eval echo -n \$$var)/" $install_script
    done
    chmod 755 "$install_script"

    # create tarball
    echo -n "${flavour}: Generating Debian APT archive file... "
    tar -C "$flavour_repo" --owner root --group root --create --gzip --file "$archive" .
    echo "Done."
}

for f in $KERNEL_FLAVOURS ; do
    make_archive $f
done
