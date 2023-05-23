"""Stage 3 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("@com_github_google_go_jsonnet//bazel:repositories.bzl", "jsonnet_go_repositories")
load("@com_github_grpc_grpc//bazel:grpc_extra_deps.bzl", "grpc_extra_deps")
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
