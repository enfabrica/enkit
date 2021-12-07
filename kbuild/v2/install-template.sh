#!/bin/sh

set -e

BASE_DIR="$(dirname $(realpath $0))"

# These variables are templated
KERNEL_FLAVOUR="%%KERNEL_FLAVOUR%%"
KERNEL_BASE="%%KERNEL_BASE%%"
DEB_VERSION="%%DEB_VERSION%%"
ARCH="%%ARCH%%"
DIST="%%DIST%%"
COMP="%%COMP%%"

KERNEL_VERSION="${KERNEL_BASE}-${KERNEL_FLAVOUR}"


usage() {
    cat<<EOF
usage: ${0##*/} OPTS

    -b Install for building kernel modules with Bazel
    -c Output APT sources.list.d config, but do not install
    -a Install APT sources.list.d config
EOF
}

# The install script is expected to output the path of the directory
# to use for bazel builds.
install_bazel_build() {

    # extract common and arch specific linux-header debs
    local install_dir="${BASE_DIR}/install"
    rm -rf "$install_dir"
    mkdir -p "$install_dir"

    local common_linux_headers_deb="${BASE_DIR}/pool/linux-headers-${KERNEL_BASE}_${DEB_VERSION}_all.deb"
    if [ ! -r "$common_linux_headers_deb" ] ; then
        echo "ERROR: unable to find common header .deb: $common_linux_headers_deb"
        exit 1
    fi

    local arch_linux_headers_deb="${BASE_DIR}/pool/linux-headers-${KERNEL_BASE}-${KERNEL_FLAVOUR}_${DEB_VERSION}_${ARCH}.deb"
    if [ ! -r "$arch_linux_headers_deb" ] ; then
        echo "ERROR: unable to find arch header .deb: $arch_linux_headers_deb"
        exit 1
    fi

    # Redirect all stdout to stderr in case any of the commands here decides to output a
    # benign informational message on stdout, breaking the build.

    {
        dpkg-deb -x "$common_linux_headers_deb" "$install_dir"
        dpkg-deb -x "$arch_linux_headers_deb" "$install_dir"

        rm -rf "${install_dir}/lib/modules/${KERNEL_VERSION}/build"
        ln -sf "${install_dir}/usr/src/linux-headers-${KERNEL_VERSION}" "${install_dir}/lib/modules/${KERNEL_VERSION}/build"
        rm -rf "${install_dir}/lib/modules/${KERNEL_VERSION}/source"
        ln -sf "${install_dir}/usr/src/linux-headers-${KERNEL_VERSION}" "${install_dir}/lib/modules/${KERNEL_VERSION}/source"

        find "$install_dir" -type f -name Makefile |xargs sed -i -e "s@/\([/a-zA-Z0-9._-]*\)usr/src/linux@\${install_dir}/usr/src/linux@g"
    } 1>&2

    build="install/lib/modules/${KERNEL_VERSION}"
    if [ -d "$build" ] ; then
        echo "$build"
        exit 0
    fi

    echo "ERROR: build directory not detected - installation failed?" 1>&2
    echo "ERROR: was looking for $build in $PWD" 1>&2
    exit 1
}

APT_SOURCES_FILE="enf-kernel-${KERNEL_VERSION}-${DIST}.list"

output_apt_config() {
    local verbose="$1"

    if [ "$verbose" = "1" ] ; then
       echo "Use this line for /etc/apt/sources.list.d/${APT_SOURCES_FILE}"
    fi
    echo "deb [arch=${ARCH} trusted=yes] copy:${BASE_DIR}/ $DIST $COMP"
    exit 0
}

install_apt_config() {
    if [ "$(id -u)" != "0" ] ; then
        echo "ERROR: sudo is required to install apt configuration"
        exit 1
    fi
    output_apt_config > "/etc/apt/sources.list.d/${APT_SOURCES_FILE}"
    exit 0
}

while getopts :bca o
do
    case $o in
        b) install_bazel_build;;
        c) output_apt_config 1;;
        a) install_apt_config;;
        *) usage; exit 1;;
    esac
done

# If no options are specified, install for a bazel build for backward
# compatibility with kernel_tree_version().
install_bazel_build
