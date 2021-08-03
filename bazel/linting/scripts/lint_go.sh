#!/usr/bin/env bash

#Golang setup
export PATH="${PWD}/${GO_LOCATION}/bin:$PATH"
SOURCE_LINT="$(find "$PWD" -name "${GO_LIBRARY_NAME}")"
export GOPATH=$SOURCE_LINT
# Store golang-ci binary
GOLANGCI_LINT=$PWD/$GOLANGCI_LINT

## this is necessary for cache + homdir lib errors
export HOME="$PWD"
mkdir -p .cache

##create output files
mkdir -p "$(dirname "${LINT_OUTPUT}")"
touch "${LINT_OUTPUT}"
export LINT_OUTPUT="$PWD/${LINT_OUTPUT}"

#fetch list of changed files from genrule
readarray -t changed_files <<< "$(cat "${GIT_DATA}")"

# $Target is the prefix of the generated gopath bundle from rules go. e.g. __astore__server_default_library
cd "$(find  "$PWD" -name "$TARGET"_source)" || exit

# Now in the generated gopath vendor find enkit
cd "$(find "$PWD" -name enfabrica)"/enkit || exit

echo "here"
# run lint, strip relative pathing and strip leading /
$GOLANGCI_LINT run ./... --exclude-use-default=false --allow-parallel-runners -D=typecheck --issues-exit-code 0 2>&1 | sed 's/.*'enkit'//' | sed -e 's/^\///' | tee "${LINT_OUTPUT}"




