#!/bin/sh

# This script builds the .debs for an Enfabrica kernel
#
# Inputs:
# - the kernel source directory
# - the kernel version file
# - the UML kernel output directory

set -e

KERNEL_DIR="$(realpath $1)"
KERNEL_VERSION="$(realpath $2)"
OUTPUT_UML_DIR="$(realpath $3)"

MOD_INSTALL="${OUTPUT_UML_DIR}/mod_install"

if [ ! -d "$KERNEL_DIR" ] ; then
    echo "ERROR: kernel build directory does not exist"
    exit 1
fi

if [ ! -r "$KERNEL_VERSION" ] ; then
    echo "ERROR: unable to read kernel version file: $KERNEL_VERSION"
    exit 1
fi

flavour="test"
kernel_version="$(cat $KERNEL_VERSION)-${flavour}"

# clean output UML dir
if [ "$RT_CLEAN_BUILD" = "yes" ] ; then
    rm -rf "$OUTPUT_UML_DIR"
fi

if [ -d "$OUTPUT_UML_DIR" ] ; then
    # skip building the UML kernel
    exit 0
fi

mkdir -p "$OUTPUT_UML_DIR"

# build the UML kernel
cd "$KERNEL_DIR"

# Use UML config from enf-linux repo
cp enfabrica/config-um "${OUTPUT_UML_DIR}/.config"

# Disable extra LOCALVERSION processing
echo "CONFIG_LOCALVERSION_AUTO=n" >> "${OUTPUT_UML_DIR}/.config"

# Disable signing external modules
echo "CONFIG_SYSTEM_TRUSTED_KEYS=\"\"" >> "${OUTPUT_UML_DIR}/.config"

# Update config with any new defaults
make ARCH=um O="$OUTPUT_UML_DIR" olddefconfig

# Build the kernel with our LOCAL version
make -j ARCH=um O="$OUTPUT_UML_DIR" LOCALVERSION="$kernel_version" all
