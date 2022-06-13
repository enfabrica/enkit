#!/bin/sh

echo ========= {message} - {target} ==========
on_exit() {
    echo $? > "/tmp/output_dir/exit_status_file" || true
    poweroff -f
}
trap on_exit EXIT

# Find the "root" where the package was mounted.
path="$0"
relpath="{relpath}"
dir="${path%%$relpath}"
test "$dir" == "$path" || cd "$dir"

set -e
{commands}
