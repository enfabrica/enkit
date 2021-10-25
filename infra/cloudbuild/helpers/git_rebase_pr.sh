#!/bin/bash
set -e

# Rebase PR onto the tip of master.
# Requires:
# * Image with `git`
# * Mounted `/root/.ssh` volume
#
# Example usage:
#  - name: gcr.io/cloud-builders/git
#    entrypoint: bash
#    args:
#      - -c
#      - infra/cloudbuild/helpers/git_rebase_pr.sh 100
#    volumes:
#      - name: ssh
#        path: /root/.ssh

# Depth to search for a common merge-base between the current commit and the
# base branch (typically master)
readonly FETCH_DEPTH="$1"

# Impersonate PR author when rebasing
# Rebasing will reauthor commits, and as no author is configured by default,
# git will complain and bail. Configure the author and email for this build
# from the last commit, which should match the PR author, as long as PRs
# don't have multiple collaborators. This way, anything that reads the commit
# author name/email won't be bamboozled by e.g. a dummy value here.
git config user.email "$(git log --format='%ae' -1)"
git config user.name "$(git log --format='%an' -1)"


# Give the PR head a local branch name
# This branch name is referenced in future rebasing steps.
git checkout -b github_pr

# Fetch more commits of the history
# We need to find a common ancestor between the PR commits and master in order
# for the rebase to succeed.  The `deepen` value likely needs to be the maximum
# of "number of commits in a PR" and "number of commits master is allowed to
# move ahead by". Potential scalability problem here: master moves at a rate
# proportional to the number of devs. So as more people join, we need to fetch
# more master to find a common ancestor.
#
# Realistically, this probably becomes "number of commits master moves in time
# period t" where t is something like one week, and then we take that position
# that PRs must be rebased at least weekly if one wants presubmits to run
# properly.
git fetch --deepen="${FETCH_DEPTH}"

# Rebase or error
# TODO(scott): Add instructions for rebasing or a pointer to such instructions
# in the error message.
readonly COMMON_ANCESTOR="$(git merge-base origin/master github_pr)"
git rebase "${COMMON_ANCESTOR}" github_pr --onto origin/master || { echo "
********************************************************************************
** Auto-rebase failure **                                                      *
********************************************************************************
* Presubmits rebase your PR onto the latest master before running, but this    *
* has failed because your PR is too out-of-date. Please rebase your PR to pick *
* updates from master and re-push.                                             *
********************************************************************************
"; /bin/false; }
