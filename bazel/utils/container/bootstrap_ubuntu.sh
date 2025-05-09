#!/bin/bash
set -o pipefail -o errexit -o errtrace
USER=$(whoami)

readonly base_image="$1"
readonly outfile="$2"
readonly preinstall_sh="$3"
readonly install_sh="$4"
readonly postinstall_sh="$5"
readonly exclude_pkgs="$6"
readonly tarballs="$7"
readonly pkgs="${@:8}"

tmp_root=$(mktemp -d)
log="$tmp_root/debootstrap/debootstrap.log"
mkdir -p "$tmp_root/downloads/pkgs" \
    "$tmp_root/dev/pts" \
    "$tmp_root/proc" \
    "$tmp_root/etc/ssl/certs/java" \
    "$tmp_root/etc/default"

cleanup() {
    if [ -e $log ]; then
        sudo cat $log
    fi
    echo "Unmounting $tmp_root/dev $tmp_root/proc $tmp_root/run/dbus"
    sudo umount -l "$tmp_root/dev"
    sudo umount -l "$tmp_root/proc"
    sudo umount -l "$tmp_root/run/dbus"
    echo "Cleaning up tmp directory $tmp_root"
    echo ""
    sudo rm -rf $tmp_root
}
trap cleanup EXIT ERR SIGINT SIGTERM

echo "Unpacking base image $base_image into $tmp_root"
echo ""
sudo tar -xf $base_image -C $tmp_root

echo "Unpacking $tarballs into $tmp_root"
echo""
IFS=','
for t in $tarballs
do
    sudo tar -xf $t -C $tmp_root
done
unset IFS

echo "Mounting /dev $tmp_root/dev for apt logging and udisks2"
echo ""
sudo mount --read-only --rbind /dev "$tmp_root/dev"

echo "Mounting /proc $tmp_root/proc for ca-certificates"
echo ""
sudo mount --read-only --rbind /proc "$tmp_root/proc"

echo "Mounting /run/dbus $tmp_root/run/dbus for packagekit"
echo ""
# https://www.reddit.com/r/linuxquestions/comments/3yrx2z/varrundbus_bind_mount_disappears_in_chroot/
sudo mkdir -p "$tmp_root/run/dbus"
sudo mount --rbind /run/dbus "$tmp_root/run/dbus"

for p in $pkgs
do
    echo "Unpacking $p into $tmp_root/downloads/pkgs"
    echo "" 
    sudo tar -xf $p -C "$tmp_root/downloads/pkgs"
done

IFS=','
for p in $exclude_pkgs
do
    sudo rm -f "$tmp_root/downloads/pkgs/$p"
done
unset IFS

echo "Copying $preinstall_sh into $tmp_root/tmp/$(basename $preinstall_sh)"
echo ""
sudo cp $preinstall_sh "$tmp_root/tmp/$(basename $preinstall_sh)"

echo "Running preinstall script $preinstall_sh"
echo""
# Configure locale language info before running the chroot script because
# the shell needs to logout then login to apply the language config.
sudo chroot $tmp_root locale-gen "en_US.UTF-8"
sudo chroot $tmp_root dpkg-reconfigure locales -f noninteractive
sudo chroot $tmp_root "/tmp/$(basename $preinstall_sh)"

echo "Copying $install_sh into $tmp_root/tmp/$(basename $install_sh)"
echo ""
sudo cp $install_sh "$tmp_root/tmp/$(basename $install_sh)"

echo "Installing additional packages under $tmp_root/downloads/pkgs"
echo ""
sudo chroot $tmp_root "/tmp/$(basename $install_sh)" "/downloads/pkgs"

echo "Copying $postinstall_sh into $tmp_root/tmp/$(basename $postinstall_sh)"
echo ""
sudo cp $postinstall_sh "$tmp_root/tmp/$(basename $postinstall_sh)"

echo "Running postinstall script $postinstall_sh"
echo""
sudo chroot $tmp_root "/tmp/$(basename $postinstall_sh)"

# The /dev, /proc, /run directories cannot be compressed into a tarball
# since these are mounts from the host.
# Exclude the /tmp directory because this contains all downloaded *.deb files
echo "Packaging bootstrap directory $tmp_root to $outfile"
echo ""
sudo chmod 0755 $tmp_root
sudo rm -rf $tmp_root/tmp/*
sudo tar --exclude="./dev" \
    --exclude="./proc" \
    --exclude="./run" -cf $outfile -C $tmp_root .
# Change ownership of outfile so that bazel doesn't complain about missing output
sudo chown $USER:$USER $outfile

