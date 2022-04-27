#!/bin/sh

# Simple script to mount what has been exposed through hostfs and then load
# all the kernel modules made available.

start() {
	echo "Running KUnit tests by loading exposing kernel modules."
	echo "... mounting what has been exposed through hostfs to /media."
	# Mount on top of /media which we know exists, since sometimes the
	# image fails to mount the rootfs as rw ==> we cannot create new
	# dirs. TODO: understand why the rootfs is being mounted as
	# read-only.
	mount none /media -t hostfs
	echo "... entering /host."
	cd /media
	echo "... searching available kernel modules."
	for KMOD in *.ko; do
		echo "... loading $KMOD."
		insmod $KMOD
	done
	echo "... done, shutting down."
	poweroff
}

case "$1" in
	start)
		$1;;
	*)
		echo "... $1 is not supported."
esac

