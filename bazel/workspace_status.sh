#!/usr/bin/env sh
# Person building the binary. If unset, assume a generic builder.
USER=${USER:-builds@enfabrica.net}

#Prints out current branch
GIT_BRANCH="$(git branch --show-current)"
echo GIT_BRANCH "$GIT_BRANCH"
# prints out the current branch with the current tracked remote from branch. e.g. origin/branch or source/branch
GIT_ORIGIN_BRANCH="$(git for-each-ref --format='%(upstream:lstrip=-2)' "$(git symbolic-ref -q HEAD)")"
echo GIT_ORIGIN_BRANCH "$GIT_ORIGIN_BRANCH"
## lists all files changed in the current remote branch, space separated
echo GIT_CHANGES "$(git --no-pager diff --name-only "$GIT_ORIGIN_BRANCH"..."$GIT_SHA" | tr '\r\n' ' ')" # list files locally modified / staged / pending
echo GIT_SHA "$(git rev-parse HEAD)" # SHA of last commit in this branch
echo GIT_AUTHOR "$(git show -s --format='%ae' $GIT_HASH)" # Author of last commit.

# If this is master, the variables below will have the same value as the variables above.
# If this is NOT master, they will track where from master this branch is derived.
echo GIT_MASTER_DISTANCE "$(git log --oneline master.."$GIT_BRANCH"|wc -l)" # Number of commits from master.
echo GIT_MASTER_SHA "$(git merge-base master $GIT_BRANCH)" # SHA of last commit on master.
echo GIT_MASTER_AUTHOR "$(git show -s --format='%ae' $GIT_MASTER_SHA)" # Author of last commit on master.
