"""Stage 4 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("@enkit_pip_deps//:requirements.bzl", python_deps = "install_deps")
load("@npm//:repositories.bzl", "npm_repositories")
load("@rules_oci//oci:pull.bzl", "oci_pull")

def stage_4():
    """Stage 4 initialization for WORKSPACE.

    This step includes any initialization which can't take place in stage 3 for
    various reasons, including:
    * A transitive load statement that references a repository that doesn't
      exist until stage 3 completes
    """

    python_deps()

    npm_repositories()

    oci_pull(
        name = "container_golang_base",
        digest = "sha256:75f63d4edd703030d4312dc7528a349ca34d48bec7bd754652b2d47e5a0b7873",
        image = "gcr.io/distroless/base",
    )
