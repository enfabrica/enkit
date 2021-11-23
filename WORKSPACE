# gazelle:repository_macro bazel/go_repositories.bzl%go_repositories

workspace(
    name = "enkit",
    managed_directories = {"@npm": ["ui/node_modules"]},
)

load("//bazel:deps.bzl", "enkit_deps")

enkit_deps()

# gazelle:repo bazel_gazelle

load("//bazel:go_repositories.bzl", "go_repositories")

# Needs to be before enkit_init() so that if there are duplicates between our
# deps and third-party tool deps instantiated in enkit_init, ours take
# precedence. Our dependencies should be strictly newer than those named by
# third-party tools.
go_repositories()

load("//bazel:init.bzl", "enkit_init")

enkit_init()

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
    name = "golang_base",
    digest = "sha256:75f63d4edd703030d4312dc7528a349ca34d48bec7bd754652b2d47e5a0b7873",
    registry = "gcr.io",
    repository = "distroless/base",
)

load("@build_bazel_rules_nodejs//:index.bzl", "yarn_install")

yarn_install(
    # Name this npm so that Bazel Label references look like @npm//package
    name = "npm",
    package_json = "//ui:package.json",
    yarn_lock = "//ui:yarn.lock",
)
