#!/bin/bash

# Script to be run after building the filesystem but before generating the
# image (to do that set the config BR2_ROOTFS_POST_BUILD_SCRIPT to point
# to this file).
# This does the following:
# - Modify the default getty prompt to a simple shell, bypassing the login
#   prompt.
# - Creates a new startup script that mounts the hostfs and loads all kernel
#   modules available there (triggering kunit tests execution).

generate_run_kunit_tests_script() {
	SCRIPT=$1
        TARGET_FS=$2
        TARGET_FILE="${TARGET_FS}/etc/init.d/S90run_kunit_tests.sh"

	cp "$SCRIPT" "$TARGET_FILE"
	chmod 777 "$TARGET_FILE"
}

remove_login_prompt() {
        TARGET_FS=$1
        TARGET_FILE="${TARGET_FS}/etc/inittab"
	sed -i 's/console::respawn:\/sbin\/getty -L  console 0 vt100 # GENERIC_SERIAL/console::respawn:-\/bin\/sh/g' "$TARGET_FILE" 
}

set -e

TARGET_FS="$1"
SCRIPT="run_kunit_tests.sh"

remove_login_prompt "$TARGET_FS"
generate_run_kunit_tests_script "$SCRIPT" "$TARGET_FS"
