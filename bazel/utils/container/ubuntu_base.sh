#!/bin/bash
set -o pipefail -o errexit -o errtrace
USER=$(whoami)

readonly arch="$1"
readonly comp="$2"
readonly distro="$3"
readonly mirror="$4"
readonly outfile="$5"
readonly pkgs="${@:6}"

tmp_root=$(mktemp -d)
log="$tmp_root/debootstrap/debootstrap.log"

cleanup() {
    if [ -e $log ]; then
        sudo cat $log
    fi
    echo "Cleaning up tmp directory $tmp_root"
    echo ""
    sudo rm -rf $tmp_root
}
trap cleanup EXIT ERR SIGINT SIGTERM

echo "Bootstrapping $distro-$arch using $mirror"
echo ""
sudo debootstrap \
    --verbose \
    --arch=$arch \
    --components=$comp \
    $distro $tmp_root $mirror

sudo chroot $tmp_root /usr/bin/apt-get update
sudo chroot $tmp_root /usr/bin/apt-get install --yes --quiet --no-install-recommends $pkgs

echo "Packaging bootstrap directory $tmp_root to $outfile"
echo ""
sudo tar -cf $outfile -C $tmp_root .
# Change ownership of outfile so that bazel doesn't complain about missing output
sudo chown $USER:$USER $outfile

