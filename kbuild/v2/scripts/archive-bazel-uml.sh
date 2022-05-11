#!/bin/sh

# This script generates a bazel tarball for building UML kernel modules
#
# Inputs:
# - the kernel source directory
# - the kernel build directory
# - the UML bazel archive output directory

set -e

KERNEL_DIR="$(realpath $1)"
OUTPUT_UML_DIR="$(realpath $2)"
OUTPUT_UML_BAZEL_ARCHIVE_DIR="$(realpath $3)"

if [ ! -d "$KERNEL_DIR" ] ; then
    echo "ERROR: kernel build directory does not exist: $KERNEL_DIR"
    exit 1
fi

if [ ! -d "$OUTPUT_UML_DIR" ] ; then
    echo "ERROR: UML kernel build directory does not exist: $OUTPUT_UML_DIR"
    exit 1
fi

INSTALL_TEMPLATE="$(dirname $(realpath $0))/../template/install-uml-bazel.sh"
if [ ! -r "$INSTALL_TEMPLATE" ] ; then
    echo "ERROR: unable to find bazel UML install script template: $INSTALL_TEMPLATE"
    exit 1
fi

KERNEL_VERSION=$(cat "${OUTPUT_UML_DIR}/include/config/kernel.release")

# clean output UML bazel archive dir
rm -rf "$OUTPUT_UML_BAZEL_ARCHIVE_DIR"
mkdir -p "$OUTPUT_UML_BAZEL_ARCHIVE_DIR"

archive="${OUTPUT_UML_BAZEL_ARCHIVE_DIR}/build-headers.tar.gz"

tmp_dir="${OUTPUT_UML_BAZEL_ARCHIVE_DIR}/tmp"
target_dir="${tmp_dir}/${KERNEL_VERSION}/build"
mkdir -p "$target_dir"

# add common files and directories to archive
for f in scripts tools arch include Makefile ; do
    cp -a "${OUTPUT_UML_DIR}/source/$f" $target_dir
done

# add build specific directories to archive
for f in scripts arch include ; do
    rsync -a "${OUTPUT_UML_DIR}/${f}/" "${target_dir}/$f"
done

# add build specific files to archive
for f in .config Module.symvers ; do
    rsync -a "${OUTPUT_UML_DIR}/${f}" "${target_dir}"
done

# create the bazel install script from a template
install_script="${tmp_dir}/install-${KERNEL_VERSION}.sh"
rm -f "$install_script"
cp  "$INSTALL_TEMPLATE" "$install_script"

# substitute some vars into the script template
for var in KERNEL_VERSION ; do
    sed -i -e "s/%%${var}%%/$(eval echo -n \$$var)/" $install_script
done
chmod 755 "$install_script"

# create tarball
echo -n "UML: Generating bazel build archive file... "
tar -C "$tmp_dir" --owner root --group root --create --gzip --file "$archive" .
rm -rf "$tmp_dir"

echo "Done."
