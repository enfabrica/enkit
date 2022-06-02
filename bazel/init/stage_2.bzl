"""Stage 2 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("//bazel:go_repositories.bzl", "go_repositories")
load("//bazel/meson:meson.bzl", "meson_register_toolchains")
load("//bazel/ui:deps.bzl", "install_ui_deps")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@bazel_skylib//:workspace.bzl", "bazel_skylib_workspace")
load("@build_bazel_rules_nodejs//:index.bzl", "node_repositories", "yarn_install")
load("@com_github_atlassian_bazel_tools//multirun:deps.bzl", "multirun_dependencies")
load("@com_github_grpc_grpc//bazel:grpc_deps.bzl", "grpc_deps")
load("@io_bazel_rules_docker//container:pull.bzl", "container_pull")
load("@io_bazel_rules_docker//go:image.bzl", rules_docker_go_dependencies = "repositories")
load("@io_bazel_rules_docker//repositories:deps.bzl", rules_docker_container_dependencies = "deps")
load("@io_bazel_rules_docker//repositories:repositories.bzl", rules_docker_dependencies = "repositories")
load("@io_bazel_rules_go//extras:embed_data_deps.bzl", "go_embed_data_dependencies")
load("@rules_foreign_cc//foreign_cc:repositories.bzl", "rules_foreign_cc_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
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
load("@rules_python//python:pip.bzl", "pip_parse")

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
    pip_parse(
        name = "python_dependencies",
        extra_pip_args = [
            # Needed for latest pytorch+CUDA install
            "--find-links=https://download.pytorch.org/whl/torch_stable.html",
            # Fixes OOMkill during torch install
            # See https://github.com/pytorch/pytorch/issues/1022
            "--no-cache-dir",
        ],
        requirements_lock = "//:requirements.txt",
    )

    go_repositories()
    go_rules_dependencies()
    go_register_toolchains(version = "1.16")
    go_embed_data_dependencies()
    gazelle_dependencies()

    rules_proto_dependencies()
    rules_proto_toolchains()

    yarn_install(
        name = "npm",
        package_json = "//ui:package.json",
        yarn_lock = "//ui:yarn.lock",
    )

    bazel_skylib_workspace()
    rules_pkg_dependencies()
    multirun_dependencies()

    install_ui_deps()

    grpc_deps()

    rules_docker_dependencies()
    rules_docker_go_dependencies()
    rules_docker_container_dependencies()

    container_pull(
        name = "golang_base",
        digest = "sha256:75f63d4edd703030d4312dc7528a349ca34d48bec7bd754652b2d47e5a0b7873",
        registry = "gcr.io",
        repository = "distroless/base",
    )

    node_repositories(
        node_version = "16.13",
        package_json = "//ui:package.json",
        yarn_version = "1.22",
    )

    rules_proto_grpc_toolchains()
    rules_proto_grpc_repos()
    grpc_web_plugin_linux()
    grpc_web_plugin_darwin()
    grpc_web_plugin_windows()

    rules_foreign_cc_dependencies()
    meson_register_toolchains()
