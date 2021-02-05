#!/usr/bin/env sh
echo "running lint script"

#Golang setup
export PATH="${PWD}/${GO_LOCATION}/bin:$PATH"
export SOURCE_LINT="$(find $PWD -name ${GO_LIBRARY_NAME})"
export GOPATH=$SOURCE_LINT

echo "running lint on directory $(find $PWD -name enfabrica)/enkit"

# this is necessary for cache + homdir lib errors
export HOME="$PWD"
mkdir -p .cache


#create output files
mkdir -p $(dirname ${LINT_OUTPUT})
touch ${LINT_OUTPUT}
export LINT_OUTPUT="$PWD/${LINT_OUTPUT}"

cd $(find $PWD -name enfabrica)/enkit
echo "$(golangci-lint run --path-prefix / --issues-exit-code 0)" >> ${LINT_OUTPUT}
cat ${LINT_OUTPUT}
