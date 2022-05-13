#!/bin/sh

echo ========= {message} - {target} ==========
# Power off the VM in case of ERR; prevent the next script from running.
trap "poweroff -f" ERR

function load {
	echo "... loading $@."
	insmod "$@"
}

# Find the "root" where the package was mounted.
path="$0"
relpath="{relpath}"
dir="${path%%$relpath}"
test "$dir" == "$path" || cd "$dir"

set -e
{commands}
