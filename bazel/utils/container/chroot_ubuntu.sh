#!/bin/bash

readonly pkg_dir="$1"
export DEBIAN_FRONTEND="noninteractive"
export DEBCONF_NONINTERACTIVE_SEEN="true"
export LANGUAGE="en_US.UTF-8"
export LC_ALL="en_US.UTF-8"
export LANG="en_US.UTF-8"

dpkg --add-architecture i386
yes | dpkg --unpack --skip-same-version --recursive $pkg_dir
yes | dpkg --install --force-configure-any --skip-same-version --recursive $pkg_dir
apt-get update
apt --yes --fix-broken install
