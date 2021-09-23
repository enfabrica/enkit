#!/usr/bin/env bats

load "external/bats_support/load.bash"
load "external/bats_assert/load.bash"

setup() {
  source bazel/tests/example.sh
}

@test "addition using bc" {
  result="$(echo 2+2 | bc)"
  [ "$result" -eq 4 ]
}

@test "find bats_assert" {
  local a
  run find -L . -print 
  assert_line --partial "external/bats_assert/load.bash"
}

@test "example_sum" {
  result="$(sum 4 5 6)"
  assert_equal "${result}" "15"
}
