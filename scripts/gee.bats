#!/usr/bin/env bats

load "external/bats_support/load.bash"
load "external/bats_assert/load.bash"

# for tputs
export TERM=screen-256color

setup() {
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
