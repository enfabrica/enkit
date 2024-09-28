#!/bin/bash
readonly pkg_dir="$1"
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export DEBIAN_FRONTEND="noninteractive"
export DEBCONF_NONINTERACTIVE_SEEN="true"

dpkg --add-architecture i386
apt-get update
yes | dpkg --unpack \
    --force-depends \
    --no-force-conflicts \
    --skip-same-version \
    --no-force-downgrade \
    --recursive $pkg_dir
yes | dpkg --install \
    --force-depends \
    --no-force-conflicts \
    --skip-same-version \
    --refuse-downgrade \
    --no-force-configure-any \
    --recursive $pkg_dir
apt --yes --fix-broken install
