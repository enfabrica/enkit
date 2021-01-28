#!/usr/bin/env sh
echo "running lint script"

#Golang setup
export PATH="${PWD}/${GO_LOCATION}/bin:$PATH"
export GOPATH=$SOURCE_LINT
export SOURCE_LINT="$(find $PWD -name ${GO_LIBRARY_NAME})"

echo "running lint on directory $(find $PWD -name enfabrica)/enkit"

# this is necessary for cache + homdir lib errors
export HOME="$PWD"
mkdir -p .cache

#run the actual linter
cd $(find $PWD -name enfabrica)/enkit
golangci-lint run --path-prefix "/"
