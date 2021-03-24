#!/usr/bin/env sh
CHANGED_FILES=$(git --no-pager diff --name-only origin/master $(git rev-parse --abbrev-ref HEAD))
GIT_SHA=$(git rev-parse HEAD)
GIT_USER=$(git config --get user.name 2>&1 || true)
GIT_EMAIL=$(git config --get user.email 2>&1 || true)
USER=$USER

echo GIT_CHANGED_FILES $CHANGED_FILES
echo GIT_SHA $GIT_SHA
echo GIT_USER $GIT_USER
echo GIT_EMAIL $GIT_EMAIL
echo USER $USER
