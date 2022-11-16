"""Stage 2 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("//bazel/meson:meson.bzl", "meson_register_toolchains")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@bazel_skylib//:workspace.bzl", "bazel_skylib_workspace")
load("@com_github_atlassian_bazel_tools//multirun:deps.bzl", "multirun_dependencies")
load("@com_github_grpc_grpc//bazel:grpc_deps.bzl", "grpc_deps")
load("@io_bazel_rules_docker//container:pull.bzl", "container_pull")
load("@io_bazel_rules_docker//go:image.bzl", rules_docker_go_dependencies = "repositories")
load("@io_bazel_rules_docker//python:image.bzl", rules_docker_python_dependencies = "repositories")
load("@io_bazel_rules_docker//repositories:deps.bzl", rules_docker_container_dependencies = "deps")
load("@io_bazel_rules_docker//repositories:repositories.bzl", rules_docker_dependencies = "repositories")
load("@io_bazel_rules_go//extras:embed_data_deps.bzl", "go_embed_data_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk", "go_register_toolchains", "go_rules_dependencies")
load("@rules_foreign_cc//foreign_cc:repositories.bzl", "rules_foreign_cc_dependencies")
load("@rules_pkg//:deps.bzl", "rules_pkg_dependencies")
load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies", "rules_proto_toolchains")
load(
    "@rules_proto_grpc//:repositories.bzl",
    "grpc_web_plugin_darwin",
    "grpc_web_plugin_linux",
    "grpc_web_plugin_windows",
    "rules_proto_grpc_repos",
    "rules_proto_grpc_toolchains",
)
load("@rules_python//python:repositories.bzl", "py_repositories", "python_register_toolchains")

def stage_2():
    """Stage 2 initialization for WORKSPACE.

    This step includes most of the rest of WORKSPACE initialization, including:
    * Loading of transitive dependencies from repositories defined in stage 1
      (unless they've been overridden with another stage 1 entry)
    * Loading of language-specific dependencies, which depend on macros/rules
      defined in their respective rules repository. For instance, pip
      dependencies are in stage 2 because they depend on the existence of
      rules_python in a load statement, which is instantiated in stage 1.
    """

    py_repositories()

    python_register_toolchains(
        name = "python3_8",
        python_version = "3.8",
        ignore_root_user_error = True,
    )

    # SDKs that can be used to build Go code. We need:
    # * the most recent version we can support
    # * the most recent version AppEngine can support (currently 1.16)
    #
    # The version of the Go SDK used during the build is the same for all
    # binaries/libraries, unless a `go_cross_binary` rule is used to specify a
    # specific version.
    #
    # The version is controlled by the
    # `--@io_bazel_rules_go//go/toolchain:sdk_version=` flag to bazel. The
    # default seems to be whichever go_download_sdk rule is listed first here.
    go_download_sdk(
        name = "go_sdk_1_20",
        version = "1.20.3",
    )

    go_rules_dependencies()
    go_register_toolchains(version = "1.20.3")

    gazelle_dependencies(go_sdk = "go_sdk_1_20")
    go_embed_data_dependencies()

    rules_proto_dependencies()
    rules_proto_toolchains()

    bazel_skylib_workspace()
    rules_pkg_dependencies()
    multirun_dependencies()

    grpc_deps()

    rules_docker_dependencies()
    rules_docker_go_dependencies()
    rules_docker_python_dependencies()
    rules_docker_container_dependencies()

    container_pull(
        name = "golang_base",
        digest = "sha256:75f63d4edd703030d4312dc7528a349ca34d48bec7bd754652b2d47e5a0b7873",
        registry = "gcr.io",
        repository = "distroless/base",
    )

    rules_proto_grpc_toolchains()
    rules_proto_grpc_repos()
    grpc_web_plugin_linux()
    grpc_web_plugin_darwin()
    grpc_web_plugin_windows()
    rules_foreign_cc_dependencies()
    meson_register_toolchains()
