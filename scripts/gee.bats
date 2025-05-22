#!/usr/bin/env bats -x

load "test_helper/bats-support/load"
load "test_helper/bats-assert/load"


setup() {
  export TERM=screen-256color
  source ./scripts/gee
}

@test "_contains_element" {
  local -a A=(1 2 3 4 5 6)
  run _contains_element 4 "${A[@]}"
  assert_success
  run _contains_element 9 "${A[@]}"
  assert_failure
}

@test "_parse_options" {
  declare -A FLAGS=()
  declare -a ARGS_POSITIONAL=()

  _parse_options "abcdef:g" -b -c -f foo bar -g -a bum

  # typeset -p FLAGS >&3
  assert_equal 1    "${FLAGS[b]}"
  assert_equal 1    "${FLAGS[c]}"
  assert_equal foo  "${FLAGS[f]}"
  assert_equal 1    "${FLAGS[g]}"
  assert_equal 1    "${FLAGS[a]}"
  assert_equal 2    "${#ARGS_POSITIONAL[@]}"
  assert_equal bar  "${ARGS_POSITIONAL[0]}"
  assert_equal bum  "${ARGS_POSITIONAL[1]}"
}

@test "_grep_for_merge_conflict_markers" {
  local testdata=$'fooo\nbar\n<<<<<<<<<\nabc\n===\nbar >>>>>>>> bum\n======\n>>>>>\n>>>>>>>>>>\n'
  run _grep_for_merge_conflict_markers <<< "${testdata}"
  assert_success
  assert_output $'<<<<<<<<<\n======\n>>>>>>>>>>'  # bats run eats the final newline.
  run _grep_for_merge_conflict_markers <<< $'foobar\ndeadbeef\n'
  assert_failure
  assert_output ""
}

@test "test_foo" {
  echo foo >&3
  echo bar >&3
}

@test "_parse_options success" {
  declare -A FLAGS=()
  declare -a ARGS_POSITIONAL=()
  run _parse_options "abcdef:g" -b -c -f foo bar -g -a bum
  assert_success
}

@test "_parse_options failure" {
  declare -A FLAGS=()
  declare -a ARGS_POSITIONAL=()
  run _parse_options "abcdef:g" -b -c -f foo bar -g -a -z bum
  assert_failure
}

@test "_gee_get_all_children_of test" {
  declare PARENTS_FILE_IS_LOADED=1
  declare -A PARENTS=(
    ["bar"]="foo"
    ["bum"]="foo"
    ["foo"]="a1"
    ["a1"]="a"
    ["echo"]="bum"
    ["delta"]="bar"
    ["charlie"]="bar"
    ["xray"]="a"
  )
  run _gee_get_all_children_of foo
  printf "got: %q\n" "$output" >&3
  assert_output $'bar\nbum\ncharlie\ndelta\necho'
}

@test "_check_pr_description checks" {
  printf "this: is a good title\n\nthis is a body\nmore body\n" > /tmp/goodpr.1
  printf "this: is a good title\nthis is a body\nmore body\n" > /tmp/badpr.1
  printf "this: is a good title\n" > /tmp/badpr.2
  printf "\nthis: is a good title\n\nfoobar\n" > /tmp/badpr.3
  run _check_pr_description /tmp/goodpr.1
  assert_success
  run _check_pr_description /tmp/badpr.1
  assert_failure
  run _check_pr_description /tmp/badpr.2
  assert_failure
  run _check_pr_description /tmp/badpr.3
  assert_failure
}

function invoke_parse_gh_pr_checks() {
  # do this in a function so that indirect variable references will work.
  declare -a CHECKS=(
    "internal-bazel-presubmit (cloud-build-290921)   fail    27m36s  https://console.cloud.google.com/cloud-build/builds/104e0ed9-6dd6-443d-8afd-1aa05c649fac?project=496137108493"
    "linter-checks (cloud-build-290921)      pass    30s     https://console.cloud.google.com/cloud-build/builds/260f3b1a-b423-4ffc-a522-5eecc6480c28?project=496137108493"
    "Pushes-preview-version-of-web-pages (cloud-build-290921)        skipping        0       https://console.cloud.google.com/cloud-build/triggers/edit/c731b7a4-47a4-4ee2-8d5e-e0f699da7568?project=496137108493"
    "external-dependencies (cloud-build-290921)      skipping        0       https://console.cloud.google.com/cloud-build/triggers/edit/4298bbb8-4289-4d32-be9e-5d0d69fe27de?project=496137108493"
  )
  declare -A CHECK_COUNTS=( [fail]=999 )
  declare -a FAILED_BUILDS=( foobar )
  _parse_gh_pr_checks CHECK_COUNTS FAILED_BUILDS "${CHECKS[@]}" >&2
  echo "RC=$?" >&2
  typeset -p CHECK_COUNTS >&2
  typeset -p FAILED_BUILDS >&2
}

@test "test _parse_gh_pr_checks" {
  run invoke_parse_gh_pr_checks
  assert_equal \
    'RC=0'$'\n''declare -A CHECK_COUNTS=([skipping]="2" [pass]="1" [fail]="1" )'$'\n''declare -a FAILED_BUILDS=([0]="104e0ed9-6dd6-443d-8afd-1aa05c649fac")' \
    "$output"
}
