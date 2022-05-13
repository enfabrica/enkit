#!/bin/sh

echo ========= {message} - {target} ==========
# NOTE: ERR instead of EXIT, otherwise it will not execute the next script
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
