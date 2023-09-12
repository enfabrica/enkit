"""Stage 4 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("@enkit_pip_deps//:requirements.bzl", python_deps = "install_deps")
load("@npm//:repositories.bzl", "npm_repositories")

def stage_4():
    """Stage 4 initialization for WORKSPACE.

    This step includes any initialization which can't take place in stage 3 for
    various reasons, including:
    * A transitive load statement that references a repository that doesn't
      exist until stage 3 completes
    """

    python_deps()

    npm_repositories()
