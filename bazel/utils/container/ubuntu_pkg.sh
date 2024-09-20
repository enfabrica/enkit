#!/bin/bash
set -o pipefail -o errexit -o nounset

USER=$(whoami)
readonly arch="$1"
readonly comp="$2"
readonly pkg="$3"
readonly distro="$4"
readonly mirror="$5"
readonly outfile="$6"

tmp_root=$(mktemp -d)
log="$tmp_root/debootstrap/debootstrap.log"
echo "Created tmp directory $tmp_root"
echo ""
pid="$$"
cleanup() {
    if [ -e $log ]; then
        sudo cat $log
    fi
    echo "Cleaning up tmp directory $tmp_root"
    echo ""
    sudo rm -rf $tmp_root
}
trap cleanup EXIT

# kill_pid() {
#     echo "Killing PID $pid"
#     echo ""
#     # Since debootstrap is executed with sudo, it must be killed with sudo
#     # since bazel does not run with root permissions.
#     sudo kill -s SIGKILL $pid
# }
# trap kill_pid SIGINT

echo "Downloading $pkg for Ubuntu $distro-$arch from $mirror"
echo ""

# debootstrap must be run as the root user
sudo debootstrap \
    --verbose \
    --make-tarball=$(realpath $outfile) \
    --arch=$arch \
    --components=$comp \
    --include=$pkg $distro $tmp_root $mirror
# Change back the ownership or else bazel
# will complain that the file was never created.
sudo chown $USER:$USER $outfile
