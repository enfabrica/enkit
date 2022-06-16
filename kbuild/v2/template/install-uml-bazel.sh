#!/bin/sh

set -e

BASE_DIR="$(dirname $(realpath $0))"

# These variables are templated
KERNEL_VERSION="%%KERNEL_VERSION%%"

# The install script is expected to output the path of the directory
# to use for bazel builds.
install_bazel_build() {

    build="${KERNEL_VERSION}/build"
    if [ -d "$build" ] ; then
        # The bazel consumer of this script will only export the
        # following directories: ["lib", "usr", "install"]
        mv "$KERNEL_VERSION" install
        echo "install"
        exit 0
    fi

    echo "ERROR: build directory not detected - installation failed?" 1>&2
    echo "ERROR: was looking for $build in $PWD" 1>&2
    exit 1
}

install_bazel_build
