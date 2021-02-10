#!/usr/bin/env sh


#Golang setup
export PATH="${PWD}/${GO_LOCATION}/bin:$PATH"
export SOURCE_LINT="$(find $PWD -name ${GO_LIBRARY_NAME})"
export GOPATH=$SOURCE_LINT
echo "go path is " $GOPATH
ls $GOPATH

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

read -a arr <<< $(cat ${GIT_DATA})

cd $(find $PWD -name enfabrica)/enkit
echo $PWD
#golangci-lint run ./...
ls lib/khttp/protocol
for i in "${arr[@]}"
do
  if [[ $i == *.go ]]; then
    go_package=$(dirname $i)
    golangci-lint run $go_package --issues-exit-code 0 2>&1 | sed 's/.*'enkit'//' | tee ${LINT_OUTPUT}
  fi
done



