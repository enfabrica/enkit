# gazelle:repository_macro bazel/go_repositories.bzl%go_repositories

workspace(
    name = "enkit",
    managed_directories = {"@npm": ["ui/node_modules"]},
)

load("//bazel/init:stage_1.bzl", "stage_1")

stage_1()

load("//bazel/init:stage_2.bzl", "stage_2")

stage_2()

load("//bazel/init:stage_3.bzl", "stage_3")

stage_3()
