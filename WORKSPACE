workspace(
    name = "enkit",
    managed_directories = {"@npm": ["node_modules"]},
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.1/rules_go-v0.23.1.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.1/rules_go-v0.23.1.tar.gz",
    ],
    sha256 = "87f0fb9747854cb76a0a82430adccb6269f7d394237104a4523b51061c469171",
)

load(
    "@io_bazel_rules_go//go:deps.bzl",
    "go_download_sdk",
    "go_register_toolchains",
    "go_rules_dependencies",
)

http_archive(
    name = "com_github_ccontavalli_bazel_rules",
    strip_prefix = "bazel-rules-6",
    urls = ["https://github.com/ccontavalli/bazel-rules/archive/v6.tar.gz"],
)

# https://golang.org/dl/?mode=json
go_download_sdk(
    name = "go_sdk",
    sdks = {
        "linux_amd64": ("go1.14.1.linux-amd64.tar.gz", "2f49eb17ce8b48c680cdb166ffd7389702c0dec6effa090c324804a5cac8a7f8"),
        "darwin_amd64": ("go1.14.1.darwin-amd64.tar.gz", "6632f9d53fd95632e431e8c34295349cca3f0a124e3a28b760ae5c42b32816e3"),
    },
)

go_rules_dependencies()

go_register_toolchains()

# Download the rules_docker repository at release v0.14.1
http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "dc97fccceacd4c6be14e800b2a00693d5e8d07f69ee187babfd04a80a9f8e250",
    strip_prefix = "rules_docker-0.14.1",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.14.1/rules_docker-v0.14.1.tar.gz"],
)

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

load("@io_bazel_rules_docker//repositories:deps.bzl", container_deps = "deps")

container_deps()

#container_pull(
#  name = "java_base",
#  registry = "gcr.io",
#  repository = "distroless/java",
#  # 'tag' is also supported, but digest is encouraged for reproducibility.
#  digest = "sha256:deadbeef",
#)
#
#load("@io_bazel_rules_docker//container:container.bzl", "container_pull")
#
#container_pull(
#    name = "alpine_linux_amd64",
#    registry = "index.docker.io",
#    repository = "library/alpine",
#    tag = "3.8",
#    digest = "sha256:954b378c375d852eb3c63ab88978f640b4348b01c1b3456a024a81536dafbbf4",
#)
#
#container_pull(
#    name = "debian_bullseye_slim_amd64",
#    registry = "index.docker.io",
#    repository = "library/debian",
#    tag = "bullseye-slim",
#    digest = "sha256:757e3d0eb1ef7d20d564c62d7937946c7a592f20a35b9bd81aec2379f056ad3c",
#)

load("@io_bazel_rules_go//extras:embed_data_deps.bzl", "go_embed_data_dependencies")

go_embed_data_dependencies()

http_archive(
    name = "build_bazel_rules_nodejs",
    sha256 = "d14076339deb08e5460c221fae5c5e9605d2ef4848eee1f0c81c9ffdc1ab31c1",
    urls = ["https://github.com/bazelbuild/rules_nodejs/releases/download/1.6.1/rules_nodejs-1.6.1.tar.gz"],
)

load("@build_bazel_rules_nodejs//:index.bzl", "yarn_install")

yarn_install(
    name = "npm",
    package_json = "//:package.json",
    yarn_lock = "//:yarn.lock",
)

load("@npm//:install_bazel_dependencies.bzl", "install_bazel_dependencies")

install_bazel_dependencies()

load("@npm_bazel_typescript//:index.bzl", "ts_setup_workspace")

ts_setup_workspace()

load("@npm_bazel_labs//:package.bzl", "npm_bazel_labs_dependencies")

npm_bazel_labs_dependencies()

http_archive(
    name = "bazel_gazelle",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/bazel-gazelle/releases/download/0.21.1/bazel-gazelle-0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/0.21.1/bazel-gazelle-0.21.1.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

http_archive(
    name = "rules_proto",
    sha256 = "602e7161d9195e50246177e7c55b2f39950a9cf7366f74ed5f22fd45750cd208",
    strip_prefix = "rules_proto-97d8af4dc474595af3900dd85cb3a29ad28cc313",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_proto/archive/97d8af4dc474595af3900dd85cb3a29ad28cc313.tar.gz",
        "https://github.com/bazelbuild/rules_proto/archive/97d8af4dc474595af3900dd85cb3a29ad28cc313.tar.gz",
    ],
)

load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies", "rules_proto_toolchains")

rules_proto_dependencies()

rules_proto_toolchains()

# gazelle:repository_macro go_repositories.bzl%go_repositories
load("//:go_repositories.bzl", "go_repositories")

go_repositories()
