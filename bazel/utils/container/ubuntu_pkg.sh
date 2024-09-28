#!/bin/bash
set -o pipefail -o errexit -o nounset

readonly arch="$1"
readonly comp="$2"
readonly pkg="$3"
readonly distro="$4"
readonly mirror="$5"
readonly exclude_pkgs="$6"
readonly outfile="$7"

tmp_root=$(mktemp -d)
log="$tmp_root/debootstrap/debootstrap.log"
echo "Created tmp directory $tmp_root"
echo ""
cleanup() {
    if [ -e $log ]; then
        cat $log
    fi
    echo "Cleaning up tmp directory $tmp_root"
    echo ""
    rm -rf $tmp_root
}
trap cleanup EXIT SIGINT SIGTERM

echo "Downloading $pkg for Ubuntu $distro-$arch from $mirror"
echo ""

fakechroot fakeroot debootstrap \
    --verbose \
    --variant=fakechroot \
    --download-only \
    --arch=$arch \
    --components=$comp \
    --include=$pkg \
    --exclude=$exclude_pkgs $distro $tmp_root $mirror

echo "Packages debs from $tmp_root/var/cache/apt/archives"
echo ""
tar -cf $outfile -C "$tmp_root/var/cache/apt/archives" .
