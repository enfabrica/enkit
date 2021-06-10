# gazelle:repository_macro bazel/go_repositories.bzl%go_repositories

workspace(
    name = "enkit",
    managed_directories = {"@npm": ["node_modules"]},
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "rules_python",
    sha256 = "778197e26c5fbeb07ac2a2c5ae405b30f6cb7ad1f5510ea6fdac03bded96cc6f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_python/releases/download/0.2.0/rules_python-0.2.0.tar.gz",
        "https://github.com/bazelbuild/rules_python/releases/download/0.2.0/rules_python-0.2.0.tar.gz",
    ],
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

http_archive(
    name = "com_github_bazelbuild_buildtools",
    strip_prefix = "buildtools-master",
    url = "https://github.com/bazelbuild/buildtools/archive/master.zip",
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "59d5b42ac315e7eadffa944e86e90c2990110a1c8075f1cd145f487e999d22b3",
    strip_prefix = "rules_docker-0.17.0",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.17.0/rules_docker-v0.17.0.tar.gz"],
)

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

load("@io_bazel_rules_docker//repositories:deps.bzl", container_deps = "deps")

container_deps()

load("@io_bazel_rules_docker//repositories:py_repositories.bzl", "py_deps")

py_deps()
load("@io_bazel_rules_docker//container:pull.bzl", "container_pull")
container_pull(
    name = "golang_base",
    digest = "sha256:75f63d4edd703030d4312dc7528a349ca34d48bec7bd754652b2d47e5a0b7873",
    registry = "gcr.io",
    repository = "distroless/base",
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
    sha256 = "94c634d499558a76fa649edb13721dce6e98fb1e7018dfaeba3cd7a083945e91",
    strip_prefix = "googletest-release-1.10.0",
    url = "https://github.com/google/googletest/archive/release-1.10.0.zip",
)
