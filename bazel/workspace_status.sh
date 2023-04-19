#!/usr/bin/env sh
#
# To learn more about status commands, read:
#    https://docs.bazel.build/versions/main/user-manual.html#workspace_status
#
# Make sure you mark variables as STABLE or not STABLE correctly.
#
# Rule of thumb:
#  - Things that always change -> not STABLE, like a timestamp, or random seed.
#  - Things that rarely change, or if they change MUST trigger a rebuild -> STABLE.
########

# IMPORTANT: some variables are used as metadata by buildbuddy. Use the same naming
# convention as described here: https://www.buildbuddy.io/docs/guide-metadata/.

# Person building the binary. Can be empty.
echo "STABLE_ENKIT_USER $(sed -e 's@.*"\(.*\)"@\1@' < ~/.config/enkit/identity/default.toml 2>/dev/null || echo $USER)"

# Prints out current branch
GIT_BRANCH="$(git branch --show-current)"
echo GIT_BRANCH "$GIT_BRANCH"

# SHA of last commit in this branch
GIT_SHA="$(git rev-parse HEAD)"
echo COMMIT_SHA "$GIT_SHA"

# prints out the current branch with the current tracked remote from branch. e.g. origin/branch or source/branch
GIT_ORIGIN_BRANCH="$(git for-each-ref --format='%(upstream:lstrip=-2)' "$(git symbolic-ref -q HEAD)")"
echo STABLE_GIT_ORIGIN_BRANCH "$GIT_ORIGIN_BRANCH"

# Author of last commit.
echo STABLE_GIT_AUTHOR "$(git show -s --format='%ae' "$GIT_SHA")"

# If this is master, the variables below will have the same value as the variables above.
# If this is NOT master, they will track where from master this branch is derived.
# Number of commits from master.
echo STABLE_GIT_MASTER_DISTANCE "$(git log --oneline master.."$GIT_BRANCH"|wc -l)"

# SHA of last commit on master.
GIT_MASTER_SHA="$(git merge-base master "$GIT_BRANCH")"
echo STABLE_GIT_MASTER_SHA "$GIT_MASTER_SHA"

# Author of last commit on master.
echo STABLE_GIT_MASTER_AUTHOR "$(git show -s --format='%ae' "$GIT_MASTER_SHA")"

# Spits out the locally changed files
echo STABLE_GIT_CHANGES "$(git status --porcelain |paste -sd "," -)"

# Spits out the files changed between this branch's remote and the default master branch. Space separated.
echo STABLE_GIT_MASTER_DIFF "$(git --no-pager diff --name-only origin/master..."$GIT_ORIGIN_BRANCH" | tr '\r\n' ' ')"
