#!/bin/sh

# This is an example script that shows how to run the scripts in order.
#
# This script assumes it is run from this directory

set -e

if [ -r ./local-config.sh ] ; then
    . ./local-config.sh
fi

# KERNEL_BRANCH: enf-linux kernel branch to build
KERNEL_BRANCH=${KERNEL_BRANCH:-enf/impish-19.19}

# KERNEL_FLAVOURS: space separated list of kernel flavours to build
KERNEL_FLAVOURS=${KERNEL_FLAVOURS:-"minimal generic"}

# BUILD_ROOT -- scratch space to perform the build
BUILD_ROOT=${BUILD_ROOT:-${HOME}/scratch/kernel-builder}

# These directories are intermediate build directories used by the
# scripts.
BUILD_DIR="$BUILD_ROOT/build"
OUTPUT_DEB_DIR="$BUILD_ROOT/deb"
OUTPUT_REPO_DIR="$BUILD_ROOT/repo"
OUTPUT_BAZEL_ARCHIVE_DIR="$BUILD_ROOT/bazel-archive"
OUTPUT_APT_ARCHIVE_DIR="$BUILD_ROOT/deb-archive"

mkdir -p $BUILD_DIR

# Builds the .deb kernel packages for all flavours
./build.sh $BUILD_DIR $KERNEL_BRANCH "$KERNEL_FLAVOURS" $OUTPUT_DEB_DIR

# Creates a portable Debian APT repository for each flavour
./repo.sh $OUTPUT_DEB_DIR "$KERNEL_FLAVOURS" $OUTPUT_REPO_DIR

# Creates a bazel ready tarball for building kernel modules
./archive-bazel.sh $OUTPUT_DEB_DIR "$KERNEL_FLAVOURS" $OUTPUT_BAZEL_ARCHIVE_DIR

# Creates a tarball of a Debian APT repository for each flavour
./archive-deb.sh $OUTPUT_DEB_DIR $OUTPUT_REPO_DIR "$KERNEL_FLAVOURS" $OUTPUT_APT_ARCHIVE_DIR

# Uploads the bazel ready tarball for each flavour
./upload.sh $OUTPUT_DEB_DIR $OUTPUT_BAZEL_ARCHIVE_DIR $OUTPUT_APT_ARCHIVE_DIR "$KERNEL_FLAVOURS" "kernel/${KERNEL_BRANCH}"

# Next update the bazel WORKSPACE with new kernel_tree_version() stanzas for each flavour
# Should be something like:
#
# kernel_tree_version(
#     name = "enf-ubuntu-impish-minimal",
#     package = "enf-5.13.0-19-1-1638665569-ga3b7c71b84a3-minimal",
#     sha256 = "157811f836bf80fdefc3fd70a9b86f7b906866cbaf23a95c1fd8245bd99ffaef",
#     url = astore_url(
#         package = "/kernel/enf/impish-19.19/minimal/kernel-artifacts.tar.gz",
#         uid = "ygwiif3nhniqjmqiugr6gnbj3746jtns",
#     ),
# )
