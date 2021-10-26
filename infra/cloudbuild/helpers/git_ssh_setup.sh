#!/bin/bash
set -e

# SSH setup for pulling repositories from Github
# More info: https://cloud.google.com/build/docs/access-github-from-build
# Requires:
# * Image with `git`
# * Mounted `/root/.ssh` volume
# * SSH_KEY defined in the environment
#
# Example usage:
#  - name: gcr.io/cloud-builders/git
#    entrypoint: bash
#    args:
#      - '-c'
#      - infra/cloudbuild/helpers/git_ssh_setup.sh enfabrica/internal
#    secretEnv:
#      - SSH_KEY
#    volumes:
#      - name: ssh
#        path: /root/.ssh

readonly REPO="$1"

# Spill the private SSH key to the root user's SSH dir
echo "${SSH_KEY}" >> /root/.ssh/id_rsa
chmod 400 /root/.ssh/id_rsa

# Copy a known_hosts file containing Github to the root user's SSH dir
cp infra/cloudbuild/helpers/known_hosts /root/.ssh/known_hosts

# Rewrite the origin URL to use SSH instead of HTTPS
git remote set-url origin "git@github.com:${REPO}"