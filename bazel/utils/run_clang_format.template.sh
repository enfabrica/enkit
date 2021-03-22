#!/bin/bash

set -e

# Retrieve the absolute path from the relative path passed by Bazel.
CLANG_FORMAT=$(realpath {clang-format})
STYLE_FILE=$(realpath {style-file})

# $BUILD_WORKSPACE_DIRECTOR is made available by Bazel to executables
# (see https://docs.bazel.build/versions/master/user-manual.html#run).
if ! cd "$BUILD_WORKSPACE_DIRECTORY"; then
  echo "Cannot cd to BUILD_WORKSPACE_DIRECTORY=${BUILD_WORKSPACE_DIRECTORY}"
  exit 1
fi

# Bazel doesn't seem to provide something like $TEST_TMPDIR to "run" rules,
# so we create our own.
export TMPDIR=$(mktemp -d)
OLD_STYLE_FILE="${TMPDIR}/.clang-format.bk"

# Backup existing .clang-format (and restore it later), otherwise we would
# overwrite it (clang-format seems to blindly assume that the style file is
# always named .clang-format).
if test -f '.clang-format'; then
	cp '.clang-format' "$OLD_STYLE_FILE"
fi

cp "${STYLE_FILE}" '.clang-format'

# Run clang-format on all the files conforming to the specified format
find . -type f \( -name '{pattern}' \) -exec "$CLANG_FORMAT" -style={style} -i {} \;

if test -f '.clang-format'; then
	cp "$OLD_STYLE_FILE" '.clang-format'
fi
