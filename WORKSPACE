# gazelle:repository_macro bazel/go_repositories.bzl%go_repositories

workspace(
    name = "enkit",
    managed_directories = {"@npm": ["ui/node_modules"]},
)

load("//bazel:deps.bzl", "enkit_deps")

enkit_deps()

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

# bats: Bash Automated Testing System
http_archive(
    name = "bats_support",
    url = "https://github.com/bats-core/bats-support/archive/refs/tags/v0.3.0.tar.gz",
    strip_prefix="bats-support-0.3.0",
    build_file = "@//bazel/dependencies:BUILD.bats_support.bazel",
    sha256 = "7815237aafeb42ddcc1b8c698fc5808026d33317d8701d5ec2396e9634e2918f",
)

http_archive(
    name = "bats_assert",
    url = "https://github.com/bats-core/bats-assert/archive/refs/tags/v2.0.0.tar.gz",
    strip_prefix="bats-assert-2.0.0",
    build_file = "@//bazel/dependencies:BUILD.bats_assert.bazel",
    sha256 = "15dbf1abb98db785323b9327c86ee2b3114541fe5aa150c410a1632ec06d9903",
)

http_archive(
    name = "bats_core",
    url = "https://github.com/bats-core/bats-core/archive/refs/tags/v1.4.1.tar.gz",
    strip_prefix="bats-core-1.4.1",
    build_file = "@//bazel/dependencies:BUILD.bats.bazel",
    sha256 = "bff517da043ae24440ec8272039f396c2a7907076ac67693c0f18d4a17c08f7d",
)


http_archive(
    name = "gtest",
    sha256 = "94c634d499558a76fa649edb13721dce6e98fb1e7018dfaeba3cd7a083945e91",
    strip_prefix = "googletest-release-1.10.0",
    url = "https://github.com/google/googletest/archive/release-1.10.0.zip",
)
