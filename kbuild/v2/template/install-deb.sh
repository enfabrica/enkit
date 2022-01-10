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

    -c     Output APT sources.list.d config, but do not install

    -a     Install APT sources.list.d config, requires sudo
EOF
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
        c) output_apt_config 1;;
        a) install_apt_config;;
        *) usage; exit 1;;
    esac
done

# If no options are specified, show the usage
usage
exit 1
