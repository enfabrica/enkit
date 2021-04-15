# gazelle:repository_macro bazel/go_repositories.bzl%go_repositories

workspace(
    name = "enkit",
    managed_directories = {"@npm": ["node_modules"]},
)

load("//bazel:deps.bzl", "enkit_deps")

enkit_deps()

load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk")

# https://golang.org/dl/?mode=json
go_download_sdk(
    name = "go_sdk",
    sdks = {
        "linux_amd64": ("go1.14.9.linux-amd64.tar.gz", "f0d26ff572c72c9823ae752d3c81819a81a60c753201f51f89637482531c110a"),
        "darwin_amd64": ("go1.14.9.darwin-amd64.tar.gz", "957926fd883998f3e212ccd422d4282be957204f89eefcf13ee2fdb730e1bab7"),
    },
)

load("//bazel:init.bzl", "enkit_init")

enkit_init()

# gazelle:repo bazel_gazelle

load("//bazel:go_repositories.bzl", "go_repositories")

go_repositories()

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "com_github_bazelbuild_buildtools",
    strip_prefix = "buildtools-master",
    url = "https://github.com/bazelbuild/buildtools/archive/master.zip",
)

load("@build_bazel_rules_nodejs//:index.bzl", "yarn_install")

yarn_install(
    # Name this npm so that Bazel Label references look like @npm//package
    name = "npm",
    package_json = "//ui/ptunnel:package.json",
    yarn_lock = "//ui/ptunnel:yarn.lock",
)

http_archive(
    name = "gtest",
    url = "https://github.com/google/googletest/archive/release-1.10.0.zip",
    strip_prefix = "googletest-release-1.10.0",
    sha256 = "94c634d499558a76fa649edb13721dce6e98fb1e7018dfaeba3cd7a083945e91",
)
