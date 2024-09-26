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

echo "Downloading $pkg for Ubuntu $distro-$arch from $mirror"
echo ""

# debootstrap must be run as the root user
sudo debootstrap \
    --verbose \
    --variant=minbase \
    --download-only \
    --arch=$arch \
    --components=$comp \
    --include=$pkg $distro $tmp_root $mirror &
pid="$!"
wait $pid

echo "Packages debs from $tmp_root/var/cache/apt/archives"
echo ""
sudo tar -cf $outfile -C "$tmp_root/var/cache/apt/archives" . &
pid="$!"
wait $pid

# Change back the ownership or else bazel
# will complain that the file was never created.
sudo chown $USER:$USER $outfile &
pid="$!"
wait $pid

# Reset the PID if all commands gracefully exit
pid=""
