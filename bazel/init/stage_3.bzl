"""Stage 3 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("@aspect_rules_js//npm:npm_import.bzl", "npm_translate_lock")
load("@com_github_bazelbuild_remote_apis//:repository_rules.bzl", "switched_rules_by_language")
load("@com_github_grpc_grpc//bazel:grpc_extra_deps.bzl", "grpc_extra_deps")
load("@google_jsonnet_go//bazel:repositories.bzl", "jsonnet_go_repositories")
load("@google_jsonnet_go//bazel:deps.bzl", "jsonnet_go_dependencies")
load("@rules_nodejs//nodejs:repositories.bzl", "DEFAULT_NODE_VERSION", "nodejs_register_toolchains")
load("@rules_oci//oci:repositories.bzl", "LATEST_CRANE_VERSION", "oci_register_toolchains")
load("@rules_oci//oci:pull.bzl", "oci_pull")
load("@rules_python//python:pip.bzl", "pip_parse")
load("@python3_8//:defs.bzl", "interpreter")

def stage_3():
    """Stage 3 initialization for WORKSPACE.

    This step includes any initialization which can't take place in stage 2 for
    various reasons, including:
    * A transitive load statement that references a repository that doesn't
      exist until stage 2 completes
    """

    pip_parse(
        name = "enkit_pip_deps",
        extra_pip_args = [
            # Needed for latest pytorch+CUDA install
            "--find-links=https://download.pytorch.org/whl/torch_stable.html",
            # Fixes OOMkill during torch install
            # See https://github.com/pytorch/pytorch/issues/1022
            "--no-cache-dir",
        ],
        requirements_lock = "//:requirements.txt",
        python_interpreter_target = interpreter,
    )

    grpc_extra_deps()

    jsonnet_go_repositories()

    jsonnet_go_dependencies()

    oci_register_toolchains(
        name = "oci",
        crane_version = LATEST_CRANE_VERSION,
    )

    oci_pull(
        name = "golang_base",
        digest = "sha256:75f63d4edd703030d4312dc7528a349ca34d48bec7bd754652b2d47e5a0b7873",
        registry = "gcr.io",
        repository = "distroless/base",
    )

    # Begin buildbarn ecosystem dependencies
    nodejs_register_toolchains(
        name = "nodejs",
        node_version = DEFAULT_NODE_VERSION,
    )

    npm_translate_lock(
        name = "npm",
        pnpm_lock = "@com_github_buildbarn_bb_storage//:pnpm-lock.yaml",
    )

    switched_rules_by_language(
        name = "bazel_remote_apis_imports",
        go = True,
    )
    # End buildbarn ecosystem dependencies
