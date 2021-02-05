#!/usr/bin/env bash
github_sha=$GIT_SHA1
github_sha=$(git rev-parse HEAD)
#if [[ -z $GIT_SHA1 ]]
#  github_sha=$(git rev-parse HEAD)
#fi
echo $PWD
ls -a
ls bazel
echo ${github_sha}
git --version
