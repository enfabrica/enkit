"""Stage 1 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:utils.bzl", "maybe")

def stage_1():
    """Stage 1 initialization for WORKSPACE.

    This step includes mostly direct dependencies. As long as this function has
    no repository rule/macro calls, and is invoked first in the WORKSPACE file,
    we can reasonably assume that each repository listed below is the version
    specified in this file, regardless of the order in which they are declared.
    Because the first WORKSPACE entry wins, any repository added to this list
    will override dependendencies loaded as part of later stages, which can be a
    way of forcing a dependency upgrade underneath e.g. io_bazel_rules_go.
    """
    maybe(
        name = "io_bazel_rules_go",
        repo_rule = http_archive,
        sha256 = "56d8c5a5c91e1af73eca71a6fab2ced959b67c86d12ba37feedb0a2dfea441a6",
        urls = [
            "https://github.com/bazelbuild/rules_go/releases/download/v0.37.0/rules_go-v0.37.0.zip",
        ],
    )

    maybe(
        name = "bazel_gazelle",
        sha256 = "448e37e0dbf61d6fa8f00aaa12d191745e14f07c31cabfa731f0c8e8a4f41b97",
        repo_rule = http_archive,
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.28.0/bazel-gazelle-v0.28.0.tar.gz",
            "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.28.0/bazel-gazelle-v0.28.0.tar.gz",
        ],
    )

    maybe(
        name = "rules_proto",
        repo_rule = http_archive,
        sha256 = "e017528fd1c91c5a33f15493e3a398181a9e821a804eb7ff5acdd1d2d6c2b18d",
        strip_prefix = "rules_proto-4.0.0-3.20.0",
        urls = [
            "https://github.com/bazelbuild/rules_proto/archive/refs/tags/4.0.0-3.20.0.tar.gz",
        ],
    )

    maybe(
        name = "bats_support",
        repo_rule = http_archive,
        url = "https://github.com/bats-core/bats-support/archive/refs/tags/v0.3.0.tar.gz",
        strip_prefix = "bats-support-0.3.0",
        build_file = "@enkit//bazel/dependencies:BUILD.bats_support.bazel",
        sha256 = "7815237aafeb42ddcc1b8c698fc5808026d33317d8701d5ec2396e9634e2918f",
    )

    maybe(
        name = "bats_assert",
        repo_rule = http_archive,
        url = "https://github.com/bats-core/bats-assert/archive/refs/tags/v2.0.0.tar.gz",
        strip_prefix = "bats-assert-2.0.0",
        build_file = "@enkit//bazel/dependencies:BUILD.bats_assert.bazel",
        sha256 = "15dbf1abb98db785323b9327c86ee2b3114541fe5aa150c410a1632ec06d9903",
    )

    maybe(
        name = "bats_file",
        repo_rule = http_archive,
        url = "https://github.com/bats-core/bats-file/archive/refs/tags/v0.2.0.tar.gz",
        strip_prefix = "bats-file-0.2.0",
        build_file = "@enkit//bazel/dependencies:BUILD.bats_file.bazel",
        sha256 = "1fa26407a68f4517cf9150d4763779ee66946a68eded33fa182ddf6a795c5062",
    )

    maybe(
        name = "bats_core",
        repo_rule = http_archive,
        url = "https://github.com/bats-core/bats-core/archive/refs/tags/v1.5.0.tar.gz",
        strip_prefix = "bats-core-1.5.0",
        sha256 = "36a3fd4413899c0763158ae194329af8f48bb1ff0d1338090b80b3416d5793af",
        patch_args = ["-p1"],
        patch_tool = "patch",
        patches = [
            "@enkit//bazel/dependencies:bats_root.patch",
        ],
        build_file = "@enkit//bazel/dependencies:BUILD.bats.bazel",
    )

    maybe(
        name = "rules_pkg",
        repo_rule = http_archive,
        urls = [
            "https://github.com/bazelbuild/rules_pkg/releases/download/0.6.0/rules_pkg-0.6.0.tar.gz",
            "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.6.0/rules_pkg-0.6.0.tar.gz",
        ],
        sha256 = "62eeb544ff1ef41d786e329e1536c1d541bb9bcad27ae984d57f18f314018e66",
    )

    maybe(
        name = "com_github_atlassian_bazel_tools",
        repo_rule = http_archive,
        strip_prefix = "bazel-tools-5c3b9306e703c6669a6ce064dd6dde69f69cba35",
        sha256 = "c8630527150f3a9594e557fdcf02694e73420c10811eb214b461e84cb74c3aa8",
        urls = [
            "https://github.com/atlassian/bazel-tools/archive/5c3b9306e703c6669a6ce064dd6dde69f69cba35.zip",
        ],
    )

    maybe(
        name = "com_google_googletest",
        repo_rule = http_archive,
        sha256 = "94c634d499558a76fa649edb13721dce6e98fb1e7018dfaeba3cd7a083945e91",
        strip_prefix = "googletest-release-1.10.0",
        url = "https://github.com/google/googletest/archive/release-1.10.0.zip",
    )

    maybe(
        name = "com_google_absl",
        repo_rule = http_archive,
        sha256 = "a4567ff02faca671b95e31d315bab18b42b6c6f1a60e91c6ea84e5a2142112c2",
        strip_prefix = "abseil-cpp-20211102.0",
        urls = ["https://github.com/abseil/abseil-cpp/archive/refs/tags/20211102.0.zip"],
    )

    maybe(
        name = "bazel_skylib",
        repo_rule = http_archive,
        sha256 = "af87959afe497dc8dfd4c6cb66e1279cb98ccc84284619ebfec27d9c09a903de",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.2.0/bazel-skylib-1.2.0.tar.gz",
            "https://github.com/bazelbuild/bazel-skylib/releases/download/1.2.0/bazel-skylib-1.2.0.tar.gz",
        ],
    )

    # TODO(INFRA-1630): Drop this patched version and use the one from enkit when we
    # can tolerate using setuptools past version 58.
    maybe(
        name = "rules_python",
        repo_rule = http_archive,
        sha256 = "29a801171f7ca190c543406f9894abf2d483c206e14d6acbd695623662320097",
        strip_prefix = "rules_python-0.18.1",
        url = "https://github.com/bazelbuild/rules_python/releases/download/0.18.1/rules_python-0.18.1.tar.gz",
    )

    maybe(
        name = "com_github_grpc_grpc",
        repo_rule = http_archive,
        patch_args = ["-p1"],
        patches = [
            "@enkit//bazel/dependencies/grpc:no_remote_tag.patch",
        ],
        sha256 = "e18b16f7976aab9a36c14c38180f042bb0fd196b75c9fd6a20a2b5f934876ad6",
        strip_prefix = "grpc-1.45.2",
        urls = [
            "https://github.com/grpc/grpc/archive/refs/tags/v1.45.2.tar.gz",
        ],
    )

    maybe(
        name = "com_github_bazelbuild_buildtools",
        repo_rule = http_archive,
        strip_prefix = "buildtools-5.1.0",
        url = "https://github.com/bazelbuild/buildtools/archive/refs/tags/5.1.0.tar.gz",
        sha256 = "e3bb0dc8b0274ea1aca75f1f8c0c835adbe589708ea89bf698069d0790701ea3",
    )

    maybe(
        name = "io_bazel_rules_docker",
        repo_rule = http_archive,
        sha256 = "27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820",
        strip_prefix = "rules_docker-0.24.0",
        urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.24.0/rules_docker-v0.24.0.tar.gz"],
        patches = [
            "@enkit//bazel/dependencies:rules_docker_no_init_go.patch",
        ],
        patch_args = ["-p1"],
    )

    maybe(
        name = "com_google_googleapis",
        repo_rule = http_archive,
        urls = ["https://github.com/googleapis/googleapis/archive/10c88bb5c489c8ad1edb0e7f6a17cdd07147966e.zip"],
        strip_prefix = "googleapis-10c88bb5c489c8ad1edb0e7f6a17cdd07147966e",
        sha256 = "e8b434794608a9af0c0721cfaeedebe37d3676a4ee9dbeed868e5e2982b5abcc",
    )

    maybe(
        name = "com_google_protobuf",
        repo_rule = http_archive,
        sha256 = "8b28fdd45bab62d15db232ec404248901842e5340299a57765e48abe8a80d930",
        strip_prefix = "protobuf-3.20.1",
        urls = ["https://github.com/protocolbuffers/protobuf/archive/refs/tags/v3.20.1.tar.gz"],
    )

    maybe(
        name = "rules_foreign_cc",
        repo_rule = http_archive,
        sha256 = "bcd0c5f46a49b85b384906daae41d277b3dc0ff27c7c752cc51e43048a58ec83",
        strip_prefix = "rules_foreign_cc-0.7.1",
        url = "https://github.com/bazelbuild/rules_foreign_cc/archive/0.7.1.tar.gz",
    )

    maybe(
        name = "meson",
        repo_rule = http_archive,
        build_file = "@enkit//bazel/meson:meson.BUILD.bazel",
        sha256 = "a0f5caa1e70da12d5e63aa6a9504273759b891af36c8d87de381a4ed1380e845",
        urls = ["https://github.com/mesonbuild/meson/releases/download/0.62.1/meson-0.62.1.tar.gz"],
        strip_prefix = "meson-0.62.1",
    )
