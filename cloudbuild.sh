#!/bin/bash

#
# This script updates bazel/enkit.versions.bzl based on the $$COMMIT
# env variable.
#
# It was written for cloudbuild, and assumes that:
#
#   1) PWD is a clone of enfabrica/enkit.
#   2) The GH_TOKEN env variable contains a token for github.
#   3) A COMMIT environment variable contains the latest enkit commit.
#
# TODO: break this down in reasonable units, allow to reuse for other automation.

ENKIT_COMMIT="$$COMMIT_SHA"
ENKIT_SHORT="$$(echo "$$COMMIT_SHA" | cut -c1-7)"

test -n "$$ENKIT_COMMIT" || {
    echo "No 'COMMIT' environment variable. Giving up. Environment:" 1>&2
    exit 5
}

# Compute the enkit SHA - from the push time to when the .tar.gz will
# be available on github it may take a few seconds, so retry for a reasonable
# number of times.

ENKIT_URL="https://github.com/enfabrica/enkit/archive/$$ENKIT_COMMIT.tar.gz"

for attempt in $$(seq 1 10); do
    echo "Attempt $$attempt to download $$ENKIT_URL..." 
    ENKIT_SHA256=$$(wget -q -O- "$$ENKIT_URL" |sha256sum |cut -d' ' -f1) && break
    sleep 10
done

test -n "$$ENKIT_SHA256" || {
    echo "Could not compute enkit SHA after 10 attempts of downloading $$ENKIT_URL - giving up" 1>&2
    exit 1
}

ENKIT_MESSAGE="$$(git log --format=%B -n 1 "$$ENKIT_COMMIT"|sed -e
's@^@    @g' || echo "COMMIT NOT FOUND")"

ENKIT_AUTHOR="$$(gh api repos/enfabrica/enkit/commits/"$$ENKIT_COMMIT"
--jq .author.login)"

test -n "$$ENKIT_AUTHOR" || {
    echo "Could not detect author of the commit" 1>&2
    exit 1
}

set -e
set -x

cd "$$(mktemp -d)"

# Configures git to work with the GH_TOKEN key.
gh auth setup-git </dev/null

# gh is interactive by desing - no great scripting integration.
#
# during testing, it would fall back to asking questions on the console
# even when not strictly necessary (eg, to ask for confirmation, or pick a default).
#
# </dev/null closes stdin, which gh detects, causing it to stop asking
# silly questions (if not all parameters are available, it fails instead).

gh repo clone enfabrica-bot/internal -- --depth=1 --single-branch </dev/null
cd "internal"

# Create a branch, and the commit.

git checkout -b "enkit-update-$$ENKIT_COMMIT" upstream/master

cat > ./bazel/enkit.version.bzl <<ENKIT_BAZEL
ENKIT_SHA = "$$ENKIT_SHA256"
ENKIT_VERSION = "$$ENKIT_COMMIT"
ENKIT_BAZEL

git add ./bazel/enkit.version.bzl
git config --global user.email "bot-email@enfabrica.net"
git config --global user.name "Enfabrica BOT"
git commit -a -F- <<COMMIT_MESSAGE

WORKSPACE: update enkit for commit $$ENKIT_SHORT

A commit was just mereged in the enkit repository.
This PR updates internal to pick up the latest version of enkit.

The [commit
merged](https://github.com/enfabrica/enkit/commit/$$ENKIT_COMMIT) in
enkit is described as:

$$ENKIT_MESSAGE

And you can blame @$$ENKIT_AUTHOR in case something breaks.

The PR is marked for auto-merge: as soon as there is an approval
and tests are passing, github will merge it automatically.

You should also know that:

- To save humans from menial work, I, enfabrica-bot, am automatically
    updating the enkit version in the internal repo.

- If I'm misbehaving, you can disable the corresponding trigger
    from cloudbuild.

- If this works well, my human overlords promised to disable the
    need for a manual approval.
COMMIT_MESSAGE

# The push -u origin below fails if the branch already exists.
#
# This is required by 'gh pr create' below - it falls back to interactive
# mode if it cannot automatically figure out the default origin.
#
# See https://github.com/cli/cli/issues/1718.

git push -u origin

gh pr create --head "enfabrica-bot:enkit-update-$$ENKIT_COMMIT" -a
"$$ENKIT_AUTHOR" -r "$$ENKIT_AUTHOR" --fill </dev/null

# This allows the PR to be automatically merged ONCE TESTS PASS and there
# is at least one approver. We may relax this constraint once proved to be working.

gh pr merge --auto --rebase </dev/null