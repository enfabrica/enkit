"""Stage 2 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("@com_github_grpc_grpc//bazel:grpc_extra_deps.bzl", "grpc_extra_deps")
load("@python_dependencies//:requirements.bzl", python_deps = "install_deps")

def stage_3():
    """Stage 3 initialization for WORKSPACE.

    This step includes any initialization which can't take place in stage 2 for
    various reasons, including:
    * A transitive load statement that references a repository that doesn't
      exist until stage 2 completes
    """
    python_deps()
    grpc_extra_deps()
