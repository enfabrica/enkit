# gazelle:repository_macro bazel/go_repositories.bzl%go_repositories

workspace(
    name = "enkit",
)

load("//bazel/init:stage_1.bzl", "stage_1")

stage_1()

load("//bazel:go_repositories.bzl", "go_overrides", "go_repositories")

# This call is placed here intentionally so that:
# * enkit and any downstream repos can keep seperate go dependencies
go_repositories()

# This call is placed here intentionally so that:
# * enkit and any downstream repos can keep seperate go dependencies
# * entries here override those in go_repositories
go_overrides()

load("//bazel/init:stage_2.bzl", "stage_2")

stage_2()

load("//bazel/init:stage_3.bzl", "stage_3")

stage_3()

load("//bazel/init:stage_4.bzl", "stage_4")

stage_4()

