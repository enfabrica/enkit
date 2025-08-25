#!/bin/sh

# This script generates a Debian APT repository from an input
# directory of .debs.
#
# Inputs:
# - a flat directory containing the kernel .debs
# - CPU architecture
# - kernel flavour
# - an output directory to place the generated APT repo

set -ex

LIB_SH="$(dirname $(realpath $0))/lib.sh"
. $LIB_SH

INPUT_DEB_ROOT="$(realpath $1)"
ARCH="$2"
FLAVOUR="$3"
OUTPUT_REPO_ROOT="$4"

DIST=focal
COMP=main

# This script only handles one flavour at a time now.
KERNEL_FLAVOURS="$FLAVOUR"

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

rm -rf "$OUTPUT_REPO_ROOT"

make_repo() {
    local flavour=$1

    # Using the input DEB root, make a pool that only contains
    # common .debs and the specified flavour.

    local flavour_repo="${OUTPUT_REPO_ROOT}/${flavour}"
    local flavour_bin_dir="${flavour_repo}/dists/${DIST}/${COMP}/binary-${ARCH}"
    local flavour_pool_dir="${flavour_repo}/pool"

    mkdir -p "$flavour_bin_dir" "$flavour_pool_dir"

    echo -n "${flavour}: Copying input files... "
    echo "KERNEL_BASE=$KERNEL_BASE"
    echo "DEB_VERSION=$DEB_VERSION"
    echo "ARCH=$ARCH"
    echo "favour=$flavour"
    find ${INPUT_DEB_ROOT}/

    cp -a "${INPUT_DEB_ROOT}/"*_all.deb "$flavour_pool_dir"
    cp -a "${INPUT_DEB_ROOT}/"*-${KERNEL_BASE}_${DEB_VERSION}_${ARCH}.deb "$flavour_pool_dir" || true
    cp -a "${INPUT_DEB_ROOT}/"*"-${flavour}"*deb "$flavour_pool_dir"
    echo "Done."

    cd "$flavour_repo"

    echo -n "${flavour}: Processing repo *.deb files... "
    apt-ftparchive --arch "$ARCH" packages pool | \
        tee "${flavour_bin_dir}/Packages" | \
        gzip > "${flavour_bin_dir}/Packages.gz"
    echo "Done."

    echo -n "${flavour}: Generating repo release file... "
    apt-ftparchive \
        --arch "$ARCH" \
        -o APT::FTPArchive::Release::Origin=Enfabrica \
        -o APT::FTPArchive::Release::Codename="$DIST" \
        -o APT::FTPArchive::Release::Architectures="$ARCH" \
        -o APT::FTPArchive::Release::Components="$COMP" \
        -o APT::FTPArchive::Release::Description='Enfabrica Kernel' \
        release "dists/${DIST}" > "dists/${DIST}/Release"
    echo "Done."

    cd - > /dev/null 2>&1
}

for f in $KERNEL_FLAVOURS ; do
    make_repo $f
done
