#!/bin/sh

echo ========= {message} - {target} ==========
on_exit() {
    echo $? > "$OUTPUT_DIR/exit_status_file" || true
    poweroff -f
}
trap on_exit EXIT

# Find the "root" where the package was mounted.
path="$0"
relpath="{relpath}"
dir="${path%%$relpath}"
test "$dir" == "$path" || cd "$dir"

set -e

mount --types tmpfs tmpfs /tmp

# Mount the output directory. This directory is shared with the host.
export OUTPUT_DIR=/tmp/output_dir
mkdir "$OUTPUT_DIR"
mount --types 9p \
    --options trans=virtio,version=9p2000.L,msize=5000000,cache=mmap,posixacl \
    /dev/output_dir "$OUTPUT_DIR"

# setup skeleton for root access
mount -t tmpfs none /var/log/
mount -t tmpfs none /root/
mkdir -p /root/.ssh
chmod 700 /root /root/.ssh

{commands}
