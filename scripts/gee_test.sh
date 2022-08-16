#!/bin/bash
#
# TODO(jonathan): fix this test now that github-playground has been deleted.

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
GEE="${SCRIPT_DIR}/gee"
TIMESTAMP="$(date +%s)"

# Clean test environment.
cd
rm -rf ~/testgee
export TESTMODE=1  # currently broken github-playground repo has been deleted.  :-()
declare ERRORS=0
declare CHECKS=0

function _cmd() {
  printf 2>&1 " %q" "$@"
  "$@"
}

function _gee() {
  _cmd "${GEE}" "$@"
}

function _expect_test {
  CHECKS=$((CHECKS+1))
  printf >&2 "EXPECT:";
  printf >&2 " %q" "$@"
  printf >&2 " ... "
  if [[ "$@" ]]; then
    printf >&2 "ok\n"
  else
    printf >&2 "ERROR\n"
    ERROR=$(( ERROR + 1 ))
  fi
}

function _expect_cmd {
  CHECKS=$((CHECKS+1))
  printf >&2 "EXPECT:";
  printf >&2 " %q" "$@"
  printf >&2 " ... "
  if "$@"; then
    printf >&2 "ok\n"
  else
    printf >&2 "ERROR\n"
    ERROR=$(( ERROR + 1 ))
  fi
}

function _expect_gee() {
  CHECKS=$((CHECKS+1))
  local RC
  _gee "$@"; RC="$?"
  if (( "${RC}" != 0 )); then
    printf >&2 "ERROR: RC=${RC}\n"
    ERROR=$((ERROR + 1))
  fi
}

_expect_gee init
_expect_test -d ~/testgee
_expect_test -d ~/testgee/.gee
_expect_test -d ~/testgee/github-playground/test-branch

cd ~/testgee/github-playground/test-branch
_expect_gee mkbr b
_expect_test -d ~/testgee/github-playground/b
cd ~/testgee/github-playground/b

mkdir geetest
mkdir "geetest/${TIMESTAMP}"
echo "a b c d e f g" > "geetest/${TIMESTAMP}/file1.txt"
echo "h i j k l m n" >> "geetest/${TIMESTAMP}/file1.txt"
_expect_gee commit -a -m "Adding file1.txt"

_expect_gee mkbr c
cd ../c
_expect_test -f "geetest/${TIMESTAMP}/file1.txt"

# cleanup
"

function test_parse_gh_pr_checks() {
  declare -a FOO=( 1 2 3 )
  declare -a CHECKS=(
    "internal-bazel-presubmit (cloud-build-290921)   fail    27m36s  https://console.cloud.google.com/cloud-build/builds/104e0ed9-6dd6-443d-8afd-1aa05c649fac?project=496137108493"
    "linter-checks (cloud-build-290921)      pass    30s     https://console.cloud.google.com/cloud-build/builds/260f3b1a-b423-4ffc-a522-5eecc6480c28?project=496137108493"
    "Pushes-preview-version-of-web-pages (cloud-build-290921)        skipping        0       https://console.cloud.google.com/cloud-build/triggers/edit/c731b7a4-47a4-4ee2-8d5e-e0f699da7568?project=496137108493"
    "external-dependencies (cloud-build-290921)      skipping        0       https://console.cloud.google.com/cloud-build/triggers/edit/4298bbb8-4289-4d32-be9e-5d0d69fe27de?project=496137108493"
  )
  declare -A CHECK_COUNTS=( [fail]=999 )
  declare -a FAILED_BUILDS=( foobar )
  _parse_gh_pr_checks CHECK_COUNTS FAILED_BUILDS "${CHECKS[@]}"
  echo RC=$? >&3
  typeset -p CHECK_COUNTS >&3
  typeset -p FAILED_BUILDS" >&3
}

@test "_parse_gh_pr_checks test" {
  run test_parse_gh_pr_checks
  assert_success
}




  #   declare -A CHECK_COUNTS=( [fail]=999 )
  #   declare -a FAILED_BUILDS=( foobar )
  #   run _parse_gh_pr_checks CHECK_COUNTS FAILED_BUILDS "${CHECKS[@]}"
  #   assert_success
  #   assert_equal  1  "${CHECK_COUNTS[fail]}"
  #   assert_equal  2  "${CHECK_COUNTS[skipping]}"
  #   assert_equal  1  "${CHECK_COUNTS[pass]}"
  #   assert_equal  1  "${#FAILED_BUILDS[@]}"
  #   assert_equal  "104e0ed9-6dd6-443d-8afd-1aa05c649fac" "${FAILED_BUILDS[0]}"
  # }
  #
