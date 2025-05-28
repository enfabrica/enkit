#!/usr/bin/env bash

set -eux

cd "${BUILD_WORKSPACE_DIRECTORY}"

bazel run @rules_go//go -- mod tidy -v
bazel run //:gazelle_update_repos
bazel mod tidy