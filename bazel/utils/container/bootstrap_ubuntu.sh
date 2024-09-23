#!/bin/bash
set -o pipefail -o errexit -o nounset
USER=$(whoami)

readonly arch="$1"
readonly comp="$2"
readonly distro="$3"
readonly mirror="$4"
readonly outfile="$5"
readonly pkgs="${@:6}"

tmp_root=$(mktemp -d)
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

for p in pkgs
do
    echo "Unpacking $p into $tmp_root/var/cache/apt/archives"
    echo "" 
    sudo tar -xf $p -C "$tmp_root/var/cache/apt/archives" &
    pid="$!"
    wait $pid
done

echo "Installing additional packages under $tmp_root/var/cache/apt/archives"
echo ""
sudo chroot $tmp_root /usr/bin/dpkg -i -R "$tmp_root/var/cache/apt/archives/" &
pid="$!"
wait $pid

echo "Packaging bootstrap directory $tmp_root to $outfile"
echo ""
sudo tar -zcf $outfile $tmp_root &
pid="$!"
wait $pid

echo "Changing ownership of $outfile to $USER:$USER"
echo ""
sudo chown $USER:$USER $outfile &
pid="$!"
wait $pid

pid=""
