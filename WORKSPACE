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

#load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
#
#http_archive(
#    name = "remote-apis",
#    strip_prefix = "remote-apis-2.3.0-rc2",
#    url = "https://github.com/bazelbuild/remote-apis/archive/refs/tags/v2.3.0-rc2.tar.gz",
#)
#
#load("@remote-apis//:repository_rules.bzl", "switched_rules_by_language")
#
#switched_rules_by_language(
#    name = "bazel_remote_apis_imports",
#    go = True,
#)
#
#http_archive(
#    name = "googleapis",
#    sha256 = "361e26593b881e70286a28065859c941e25b96f9c48ba91127293d0a881152d6",
#    strip_prefix = "googleapis-a3770599794a8d319286df96f03343b6cd0e7f4f",
#    urls = ["https://github.com/googleapis/googleapis/archive/a3770599794a8d319286df96f03343b6cd0e7f4f.zip"],
#)
#
#load("@googleapis//:repository_rules.bzl", "switched_rules_by_language")
#
#switched_rules_by_language(
#    name = "com_google_googleapis_imports",
#    go = True,
#)
