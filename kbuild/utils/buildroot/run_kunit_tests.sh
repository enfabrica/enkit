#!/bin/sh

# Simple script to mount what has been exposed through hostfs and then load
# all the kernel modules made available.

start() {
	trap "poweroff -f" EXIT

	echo "Running KUnit tests by loading kernel modules."
	echo "... mounting what has been exposed through hostfs to /media."
	# Using /media that is guaranteed to exist in the rootfs.
	mount none /media -t hostfs
	echo "... entering /host."
	cd /media || {
		echo "... FAILED TO cd /media"
		exit 20
        }
	test -x /media/init.sh && {
		echo "... found init.sh, execing it"
		exec /media/init.sh
	}

	echo "... searching available kernel modules."
	for KMOD in *.ko; do
		test -e "$KMOD" || {
			echo "... NO MODULES FOUND!"
			exit 10
		}

		echo "... loading $KMOD."
		insmod "$KMOD"
	done
	echo "... done, shutting down."
}

case "$1" in
	start)
		start;;
	*)
		echo "... $1 is not supported."
esac

