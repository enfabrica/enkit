"""Stage 2 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("@aspect_bazel_lib//lib:repositories.bzl", "aspect_bazel_lib_dependencies", "aspect_bazel_lib_register_toolchains")
load("@rules_distroless//distroless:dependencies.bzl", "distroless_dependencies")
load("@aspect_rules_js//js:repositories.bzl", "rules_js_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@bazel_skylib//:workspace.bzl", "bazel_skylib_workspace")
load("@com_github_atlassian_bazel_tools//multirun:deps.bzl", "multirun_dependencies")
load("@com_github_grpc_grpc//bazel:grpc_deps.bzl", "grpc_deps")
load("@googleapis//:repository_rules.bzl", "switched_rules_by_language")
load("@io_bazel_rules_go//extras:embed_data_deps.bzl", "go_embed_data_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk", "go_register_toolchains", "go_rules_dependencies")
load("@io_bazel_rules_jsonnet//jsonnet:jsonnet.bzl", "jsonnet_repositories")
load("@rules_antlr//antlr:repositories.bzl", "rules_antlr_dependencies")
load("@rules_foreign_cc//foreign_cc:repositories.bzl", "rules_foreign_cc_dependencies")
load("@rules_oci//oci:dependencies.bzl", "rules_oci_dependencies")
load("@rules_pkg//:deps.bzl", "rules_pkg_dependencies")
load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies")
load("@rules_proto_grpc//:repositories.bzl", "rules_proto_grpc_repos", "rules_proto_grpc_toolchains")
load("@rules_python//python:repositories.bzl", "py_repositories", "python_register_toolchains")
load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")
load("@bazel_features//:deps.bzl", "bazel_features_deps")

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

    rules_foreign_cc_dependencies()
    bazel_skylib_workspace()
    bazel_features_deps()
    py_repositories()

    aspect_bazel_lib_dependencies()
    aspect_bazel_lib_register_toolchains()

    distroless_dependencies()

    python_register_toolchains(
        name = "python3_12",
        python_version = "3.12",
        ignore_root_user_error = True,
    )

    # Workaround for a bug where rules_python doesn't register
    # the python c toolchain ("py cc").
    # see https://github.com/bazelbuild/rules_python/issues/1669
    # and https://rules-python.readthedocs.io/en/latest/toolchains.html#python-c-toolchain-type
    native.register_toolchains("@python3_12_toolchains//:all")
    
    protobuf_deps()

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
        name = "go_sdk_1_21",
        version = "1.21.4",
    )

    go_rules_dependencies()
    go_register_toolchains()

    gazelle_dependencies(go_sdk = "go_sdk_1_21")
    go_embed_data_dependencies()

    rules_proto_grpc_repos()
    rules_proto_grpc_toolchains()

    rules_proto_dependencies()

    rules_pkg_dependencies()
    multirun_dependencies()

    # IMPORTANT: grpc_deps() pulls in boringssl as a WORKSPACE dependency. In order to apply patches
    # to boringssl, we define boringssl BEFORE grpc_deps is invoked - that way, our version will be picked
    # over the default one.
    # You must manually update boringssl when grpc is updated. If ARM support is added upstream, we may
    # be able to remove the patches and the work here.
    grpc_deps()

    rules_oci_dependencies()

    # Begin transitive deps required by deps of buildbarn ecosystem
    switched_rules_by_language(
        name = "com_google_googleapis_imports",
        python = True,
    )
    jsonnet_repositories()
    rules_js_dependencies()
    rules_antlr_dependencies("4.10")
    # End transitive deps required by deps of buildbarn ecosystem
