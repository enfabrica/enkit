#!/bin/bash
set -o pipefail -o errexit -o nounset
USER=$(whoami)

readonly arch="$1"
readonly comp="$2"
readonly distro="$3"
readonly mirror="$4"
readonly outfile="$5"
readonly bootstrap_tar="$6"

tmp_dir=$(sudo mktemp -d)
sudo mkdir -p "$tmp_dir/root"
tmp_root="$tmp_dir/root"
log="$tmp_root/debootstrap/debootstrap.log"
cleanup() {
    if [ -e $log ]; then
        sudo cat $log
    fi
    echo "Cleaning up tmp directory $tmp_dir"
    echo ""
    sudo rm -rf $tmp_dir
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

sudo debootstrap \
    --verbose \
    --unpack-tarball=$(realpath $bootstrap_tar) \
    --arch=$arch \
    --components=$comp \
    $distro $tmp_root $mirror
sudo tar -zcf $outfile $tmp_root
sudo chown $USER:$USER $outfile

