#!/bin/bash
set -o pipefail -o errexit -o nounset
USER=$(whoami)

readonly outfile="$1"
readonly pkgs="${@:2}"
tmp_dir=$(mktemp -d)
pid=""
cleanup() {
    echo "Cleaning up tmp directory $tmp_dir"
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
    sudo rm -rf $tmp_dir
}
trap cleanup EXIT

for p in $pkgs
do
    echo "Unpacking $p into $tmp_dir"
    echo ""
    tar -xf $p -C $tmp_dir "var/cache/apt/archives"
done

tar -zcf $outfile -C $tmp_dir .
# Change the ownership of the file back to a regular user
# or else bazel will fail because bazel does not treat
# output files owned by root as valid.
#sudo chown $USER:$USER $outfile

