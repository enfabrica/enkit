#!/usr/bin/env sh
# Person building the binary. If unset, assume a generic builder.
USER=${USER:-builds@enfabrica.net}

#Prints out current branch
GIT_BRANCH="$(git branch --show-current)"
echo GIT_BRANCH "$GIT_BRANCH"

# SHA of last commit in this branch
GIT_SHA="$(git rev-parse HEAD)"
echo GIT_SHA "$GIT_SHA"

# prints out the current branch with the current tracked remote from branch. e.g. origin/branch or source/branch
GIT_ORIGIN_BRANCH="$(git for-each-ref --format='%(upstream:lstrip=-2)' "$(git symbolic-ref -q HEAD)")"
echo GIT_ORIGIN_BRANCH "$GIT_ORIGIN_BRANCH"

# Author of last commit.
echo GIT_AUTHOR "$(git show -s --format='%ae' "$GIT_SHA")"

# If this is master, the variables below will have the same value as the variables above.
# If this is NOT master, they will track where from master this branch is derived.
# Number of commits from master.
echo GIT_MASTER_DISTANCE "$(git log --oneline master.."$GIT_BRANCH"|wc -l)"

# SHA of last commit on master.
GIT_MASTER_SHA="$(git merge-base master "$GIT_BRANCH")"
echo GIT_MASTER_SHA "$GIT_MASTER_SHA"

# Author of last commit on master.
echo GIT_MASTER_AUTHOR "$(git show -s --format='%ae' "$GIT_MASTER_SHA")"

# Spits out the locally changed files
echo GIT_CHANGES "$(git status --porcelain |paste -sd "," -)"

# Spits out the files changed between this branch's remote and the default master branch. Space separated.
echo GIT_MASTER_DIFF "$(git --no-pager diff --name-only "$GIT_ORIGIN_BRANCH"...origin/master | tr '\r\n' ' ')"
