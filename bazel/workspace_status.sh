#!/usr/bin/env sh
# Person building the binary. If unset, assume a generic builder.
USER=${USER:-builds@enfabrica.net}

# Is this building master? Are there local changes?
GIT_BRANCH="$(git branch --show-current)"
echo GIT_BRANCH "$GIT_BRANCH"
echo GIT_CHANGES "$(git --no-pager diff --name-only origin/master "$(git rev-parse --abbrev-ref HEAD)")" # list files locally modified / staged / pending
echo GIT_SHA "$(git rev-parse HEAD)" # SHA of last commit in this branch
echo GIT_AUTHOR "$(git show -s --format='%ae' $GIT_HASH)" # Author of last commit.

# If this is master, the variables below will have the same value as the variables above.
# If this is NOT master, they will track where from master this branch is derived.
echo GIT_MASTER_DISTANCE "$(git log --oneline master.."$GIT_BRANCH"|wc -l)" # Number of commits from master.
echo GIT_MASTER_SHA "$(git merge-base master $GIT_BRANCH)" # SHA of last commit on master.
echo GIT_MASTER_AUTHOR "$(git show -s --format='%ae' $GIT_MASTER_SHA)" # Author of last commit on master.
