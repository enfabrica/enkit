"""Stage 4 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("@enkit_pip_deps//:requirements.bzl", python_deps = "install_deps")
load("@npm//:repositories.bzl", "npm_repositories")
load("@rules_oci//oci:pull.bzl", "oci_pull")
load("@io_bazel_rules_docker//container:pull.bzl", "container_pull")

def stage_4():
    """Stage 4 initialization for WORKSPACE.

    This step includes any initialization which can't take place in stage 3 for
    various reasons, including:
    * A transitive load statement that references a repository that doesn't
      exist until stage 3 completes
    """

    python_deps()

    npm_repositories()

    container_pull(
        name = "container_golang_base",
        digest = "sha256:a4eefd667af74c5a1c5efe895a42f7748808e7f5cbc284e0e5f1517b79721ccb",
        registry = "us-docker.pkg.dev",
        repository = "enfabrica-container-images/third-party-prod/distroless/base/golang",
    )

    container_pull(
        name = "golang_base",
        digest = "sha256:a4eefd667af74c5a1c5efe895a42f7748808e7f5cbc284e0e5f1517b79721ccb",
        registry = "us-docker.pkg.dev",
        repository = "enfabrica-container-images/third-party-prod/distroless/base/golang",
    )

