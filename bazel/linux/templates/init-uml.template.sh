#!/bin/sh

echo ========= {message} - {target} ==========
trap "poweroff -f" EXIT

# Find the "root" where the package was mounted.
path="$0"
relpath="{relpath}"
dir="${path%%$relpath}"
test "$dir" == "$path" || cd "$dir"

set -e
{inits}
{commands}
