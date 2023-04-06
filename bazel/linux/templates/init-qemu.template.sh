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

# Remount / with the nosuid flag. Otherwise the following mount fails with:
# mount: only root can use "--options" option (effective UID is 65534)
python -c 'import ctypes; exit(ctypes.cdll.LoadLibrary("libc.so.6").mount("", "/", "", 2|32, 0))'

mount --types tmpfs tmpfs /tmp

# Mount the output directory. This directory is shared with the host.
export OUTPUT_DIR=/tmp/output_dir
mkdir "$OUTPUT_DIR"
mount --types 9p \
    --options trans=virtio,version=9p2000.L,msize=5000000,cache=mmap,posixacl \
    /dev/output_dir "$OUTPUT_DIR"

{inits}

{commands}
