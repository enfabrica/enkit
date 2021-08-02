#!/usr/bin/env bash


#Golang setup
export PATH="${PWD}/${GO_LOCATION}/bin:$PATH"
export SOURCE_LINT="$(find $PWD -name ${GO_LIBRARY_NAME})"
export GOPATH=$SOURCE_LINT
echo "go path is " $GOPATH
ls $GOPATH
GOLANGCI_LINT=$PWD/$GOLANGCI_LINT
echo "running lint on directory $(find $PWD -name enfabrica)/enkit"
#
## this is necessary for cache + homdir lib errors
export HOME="$PWD"
mkdir -p .cache

#
#
##create output files
mkdir -p $(dirname ${LINT_OUTPUT})
touch ${LINT_OUTPUT}
export LINT_OUTPUT="$PWD/${LINT_OUTPUT}"

#fetch list of changed files from genrule
readarray -t changed_files <<< "$(cat "${GIT_DATA}")"
echo "Changed files are ${changed_files[*]}"
cd "$(find "$PWD" -name enfabrica)"/enkit || exit

$GOLANGCI_LINT run ./... --issues-exit-code 0 2>&1 | sed 's/.*'enkit'//' | sed -e 's/^\///' |tee ${LINT_OUTPUT}
#$GOLANGCI_LINT run ./... --issues-exit-code 0 2>&1 | tee ${LINT_OUTPUT}
##golangci-lint run ./...
#for i in "${changed_files[@]}"
#do
#  if [[ $i == *.go ]]; then
#    go_package=$(dirname $i)
#    $GOLANGCI_LINT run "$go_package" --verbose --issues-exit-code 0 2>&1 | sed 's/.*'enkit'//' | tee ${LINT_OUTPUT}
#  fi
#done



