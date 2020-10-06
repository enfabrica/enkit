#!/bin/bash


INSTALLED=${INSTALLED:-$PWD}

# The install script is expected to output the path of the directory to use for builds.
# Redirect all stdout to stderr in case any of the commands here decides to output a
# benign informational message on stdout, breaking the build.

{
    set -e

    {{range .Symlinks}}
    rm "$INSTALLED"/'{{.Path}}'
    ln -sf "$INSTALLED"/'{{.Target}}' "$INSTALLED"/'{{.Path}}'
    {{end}}
    
    find . -type f -name Makefile |xargs sed -i -e "s@/\([/a-zA-Z0-9._-]*\)usr/src/linux@$INSTALLED/usr/src/linux@g"
} 1>&2

build='lib/modules/{{.Version.Id}}'
test -d "$build" && { echo "$build"; exit 0; }
build='lib/modules/{{.Version.ArchLessId}}'
test -d "$build" && { echo "$build"; exit 0; }

echo "build directory not detect - installation failed?" 1>&2
echo "was looking for '$build' in $PWD" 1>&2
exit 1
