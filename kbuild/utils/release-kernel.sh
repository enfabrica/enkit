#!/usr/bin/env bash
#
# Script used to release a custom kernel to Enfabrica's artifact store.
#
# Takes a branch name and sha1/tag pertaining to the enfabrica/linux repository
# on Github as input, builds the base kernel, a VM kernel, and User-Mode Linux
# (used for testing), and uploads the archives to astore.
#

set -e

# Builds for the same x.y.z semver'ed kernel at different tags are versioned in
# astore using the provided tag, so the variables here need not be changed for
# each build under the same branch.
ASTORE_BASE_KERNEL_TREE=${ASTORE_BASE_KERNEL_TREE:-"enf-kernel.tar.gz"}
ASTORE_TEST_KERNEL_TREE=${ASTORE_TEST_KERNEL_TREE:-"enf-test-um.tar.gz"}
ASTORE_TEST_KERNEL_IMG=${ASTORE_TEST_KERNEL_IMG:-"enf-uml-img"}
KBUILD_UTILS="$(git rev-parse --show-toplevel)/kbuild/utils"

# Ephemeral dirs used for builds, cleaned up on exit.
LINUX_SOURCE="$(mktemp -d)"
LINUX_BASE_BUILD="$(mktemp -d)"
LINUX_VM_BUILD="$(mktemp -d)"
LINUX_TEST_BUILD="$(mktemp -d)"
cleanup() {
	rm -rf ${LINUX_TEST_BUILD}
	rm -rf ${LINUX_VM_BUILD}
	rm -rf ${LINUX_BASE_BUILD}
	rm -rf ${LINUX_SOURCE}
}
trap cleanup EXIT

usage() {
	cat <<EOF

USAGE:
	${0} <OPTIONS>

OPTIONS:
	-b branch
		Git branch to release from. (Mandatory)
	-t tag
		Git sha1 or tag, defaults to current tip of branch. (Mandatory)
	-p path
		Astore deployment path prefix, defaults to kernel/enf/<kernel-version>.
	-v verbose
		Enable verbose tracing of the script for debug purposes.

EOF
}

prep_linux_source() {
	# TODO: Bazel-ify this by taking a git_repository dependency on this and setup a genrule;
	git clone --depth=100 --single-branch -b ${RELEASE_BRANCH} git@github.com:enfabrica/linux.git ${LINUX_SOURCE}
	pushd ${LINUX_SOURCE}/
	git rev-parse --quiet --verify "${RELEASE_TAG}" || { echo "Invalid or ambiguous tag provided."; exit 1; }
	popd
}

# $1: KBUILD_CONFIG
# $2: ARCH
build_kernel() {
	pushd ${LINUX_SOURCE}/
	make mrproper

	# We will explicitly update the config when updating the kernel if
	# necessary, just get to parity with any new defaults
	cp $1 .config
	make olddefconfig $2
	${PWD}/enfabrica/build-kernel.sh -b ${PWD} -- -j16 all $2
	popd
}

# $1: Astore upload dir
# $2: ARCH
push_astore_kernel() {
	${KBUILD_UTILS}/generate_custom_archive.sh -k ${LINUX_SOURCE} -t ${LINUX_BASE_BUILD}

	# TODO: Assume the operator has authenticated with enkit before calling
	# this script. We are going to need some graceful auth solution when we
	# hook this up to Cloudbuild/Github actions.
	mv ${LINUX_BASE_BUILD}/*.tar.gz ${LINUX_BASE_BUILD}/${ASTORE_BASE_KERNEL_TREE}
	enkit astore upload -d $1 "${LINUX_BASE_BUILD}/${ASTORE_BASE_KERNEL_TREE}" -a $2 -t ${RELEASE_TAG}

	# astore publish commands are not idempotent, so the script can not
	# add an object each time it is updated. The explicit `del` has to be
	# removed once astore can support directory publishing, which will allow
	# a one-time publish rule config for ${ASTORE_PATH}.
	enkit astore public del "$1/${ASTORE_BASE_KERNEL_TREE}"
	enkit astore public add "$1/${ASTORE_BASE_KERNEL_TREE}" -a $2
}

# Argument parsing
while getopts ":b:t:p:hv" opt; do
	case ${opt} in
		b) RELEASE_BRANCH="${OPTARG}";;
		t) RELEASE_TAG="${OPTARG}";;
		p) ASTORE_PATH="${OPTARG}";;
		h) usage; exit 1;;
		v) set -x;;
		\?) echo "Unknown option: -${OPTARG}"; usage; exit 1;;
		:) echo "Missing argument for -${OPTARG}"; usage; exit 1;;
	esac
done
shift $((OPTIND - 1))
if [ -z "${RELEASE_BRANCH}" ] || [ -z "${RELEASE_TAG}" ]; then
	echo "-b and -t are mandatory arguments."
	exit 1
fi

ASTORE_PATH=${ASTORE_PATH:-"kernel/${RELEASE_BRANCH}"}

prep_linux_source

# Base kernel with all goodies
build_kernel "${LINUX_SOURCE}/enfabrica/config/config-amd64"
push_astore_kernel "${ASTORE_PATH}" "amd64"

# A stripped down kernel for just a devel VM
build_kernel "${LINUX_SOURCE}/enfabrica/config/config-vm"
push_astore_kernel "${ASTORE_PATH}/vm" "amd64"

# UML kernel used by Bazel builds
build_kernel "${LINUX_SOURCE}/enfabrica/config/config-um" "ARCH=um"
push_astore_kernel "${ASTORE_PATH}/test" "um"

# Upload the UML `linux` image too, which is used by the tests.
enkit astore upload "${LINUX_SOURCE}/linux"@"${ASTORE_PATH}/test/${ASTORE_TEST_KERNEL_IMG}" -a um -t ${RELEASE_TAG}
enkit astore public del "${ASTORE_PATH}/test/${ASTORE_TEST_KERNEL_IMG}"
enkit astore public add "${ASTORE_PATH}/test/${ASTORE_TEST_KERNEL_IMG}" -a um
