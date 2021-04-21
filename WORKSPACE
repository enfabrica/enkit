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

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "4521794f0fba2e20f3bf15846ab5e01d5332e587e9ce81629c7f96c793bb7036",
    strip_prefix = "rules_docker-0.14.4",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.14.4/rules_docker-v0.14.4.tar.gz"],
)
load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

load("@io_bazel_rules_docker//repositories:deps.bzl", container_deps = "deps")

container_deps()

load("@io_bazel_rules_docker//repositories:pip_repositories.bzl", "pip_deps")

pip_deps()
load("@io_bazel_rules_docker//container:pull.bzl", "container_pull")
container_pull(
    name = "ubuntu-20.04",
    digest = "sha256:2e70e9c81838224b5311970dbf7ed16802fbfe19e7a70b3cbfa3d7522aa285b4",
    registry = "index.docker.io",
    repository = "library/ubuntu",
    tag = "20.04",
)

