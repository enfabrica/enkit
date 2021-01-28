#!/usr/bin/env sh
echo "running lint script"
#sleep 5
echo "hello hello" >> "lint_result.txt"
#ls bazel-out/darwin-fastbuild/bin/astore/client/astore_source/src/github.com/enfabrica/enkit/astore
#if [[ -nz $1 ]]
#  echo "must provide path prefix"
#fi
export HOME="$PWD"
mkdir -p .cache
echo "here"
#ls
#echo ${enkit}
#export PATH=$PATH:"${enkit}"
#bash -c "${enkit} --help"
#echo "here again "
golangci-lint run --path-prefix bazel-out/darwin-fastbuild/bin/astore/client/astore_source/src/github.com/enfabrica/enkit
