#!/bin/bash
set -o pipefail -o errexit -o nounset
USER=$(whoami)

readonly arch="$1"
readonly comp="$2"
readonly distro="$3"
readonly mirror="$4"
readonly outfile="$5"
readonly chroot_sh="$6"
readonly pkgs="${@:7}"

tmp_root=$(mktemp -d)
mkdir -p "$tmp_root/var/cache/apt/archives"
mkdir -p "$tmp_root/tmp"
mkdir -p "$tmp_root/dev/pts"
log="$tmp_root/debootstrap/debootstrap.log"
pid=""
cleanup() {
    if [ -e $log ]; then
        sudo cat $log
    fi
    echo "Cleaning up tmp directory $tmp_root"
    echo ""
    # The bazel parent process does not have
    # permission to kill a child process running as sudo.
    # Manually kill the sudo child process so that
    # when a user sends CTRL+C, the child process is killed.
    if [ -n "$pid" ]; then
        echo "Killing PID $pid"
        echo ""
        sudo kill --signal SIGKILL $pid
    fi
    if [[ $(mount | grep "$tmp_root/dev/pts") != "" ]]; then
        echo "Unmounting $tmp_root/dev/pts"
        sudo umount -f "$tmp_root/dev/pts"
    fi
    sudo rm -rf $tmp_root
}
trap cleanup EXIT SIGINT SIGTERM

echo "Bootstrapping $distro-$arch using $mirror"
echo ""
sudo debootstrap \
    --verbose \
    --arch=$arch \
    --components=$comp \
    $distro $tmp_root $mirror &
pid="$!"
wait $pid

for p in $pkgs
do
    echo "Unpacking $p into $tmp_root/var/cache/apt/archives"
    echo "" 
    sudo tar -xf $p -C "$tmp_root/var/cache/apt/archives" &
    pid="$!"
    wait $pid
done

echo "Copying $chroot_sh into $tmp_root/tmp/$(basename $chroot_sh)"
echo ""
sudo cp $chroot_sh "$tmp_root/tmp/$(basename $chroot_sh)" &
pid="$!"
wait $pid

echo "Mounting /dev/pts into $tmp_root/dev/pts for apt logging"
echo ""
sudo mount --bind /dev/pts "$tmp_root/dev/pts" &
pid="$!"
wait $pid

echo "Installing additional packages under $tmp_root/var/cache/apt/archives"
echo ""
sudo chroot $tmp_root "/tmp/$(basename $chroot_sh)" "/var/cache/apt/archives" &
pid="$!"
wait $pid

echo "Packaging bootstrap directory $tmp_root to $outfile"
echo ""
sudo tar -zcf $outfile -C $tmp_root . &
pid="$!"
wait $pid

echo "Changing ownership of $outfile to $USER:$USER"
echo ""
sudo chown $USER:$USER $outfile &
pid="$!"
wait $pid

pid=""
