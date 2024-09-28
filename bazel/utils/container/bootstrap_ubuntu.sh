#!/bin/bash
set -o pipefail -o errexit -o errtrace -o nounset
USER=$(whoami)

readonly arch="$1"
readonly comp="$2"
readonly distro="$3"
readonly mirror="$4"
readonly outfile="$5"
readonly chroot_sh="$6"
readonly exclude_pkgs="$7"
readonly pkgs="${@:8}"

tmp_root=$(mktemp -d)
log="$tmp_root/debootstrap/debootstrap.log"
mkdir -p "$tmp_root/var/cache/apt/archives" \
    "$tmp_root/tmp" \
    "$tmp_root/dev/pts" \
    "$tmp_root/proc" \
    "$tmp_root/etc/ssl/certs/java" \
    "$tmp_root/etc/default"

cleanup() {
    if [ -e $log ]; then
        sudo cat $log
    fi
    echo "Unmounting $tmp_root/dev $tmp_root/proc"
    sudo umount -l "$tmp_root/dev"
    sudo umount -l "$tmp_root/proc"
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

echo "Mounting /dev $tmp_root/dev for apt logging and udisks2"
echo ""
sudo mount --read-only --rbind /dev "$tmp_root/dev"

echo "Mounting /proc $tmp_root/proc for ca-certificates"
echo ""
sudo mount --read-only --rbind /proc "$tmp_root/proc"

for p in $pkgs
do
    echo "Unpacking $p into $tmp_root/var/cache/apt/archives"
    echo "" 
    sudo tar -xf $p -C "$tmp_root/var/cache/apt/archives"
done

IFS=','
for p in $exclude_pkgs
do
    sudo rm -f "$tmp_root/var/cache/apt/archives/$p"
done
unset IFS

echo "Copying $chroot_sh into $tmp_root/tmp/$(basename $chroot_sh)"
echo ""
sudo cp $chroot_sh "$tmp_root/tmp/$(basename $chroot_sh)"


echo "Installing additional packages under $tmp_root/var/cache/apt/archives"
echo ""
# Configure locale language info before running the chroot script because
# the shell needs to logout then login to apply the language config.
sudo chroot $tmp_root locale-gen "en_US.UTF-8"
sudo chroot $tmp_root dpkg-reconfigure locales -f noninteractive
sudo chroot $tmp_root "/tmp/$(basename $chroot_sh)" "/var/cache/apt/archives"

# The /dev and /proc directories cannot be compressed into a tarball
# since these are mounts from the host.
echo "Packaging bootstrap directory $tmp_root to $outfile"
echo ""
sudo tar --exclude="./dev" --exclude="./proc" -zcf $outfile -C $tmp_root .
# Change ownership of outfile so that bazel doesn't complain about missing output
sudo chown $USER:$USER $outfile

