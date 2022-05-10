"""Stage 1 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

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
    http_archive(
        name = "io_bazel_rules_go",
        sha256 = "d6b2513456fe2229811da7eb67a444be7785f5323c6708b38d851d2b51e54d83",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.30.0/rules_go-v0.30.0.zip",
            "https://github.com/bazelbuild/rules_go/releases/download/v0.30.0/rules_go-v0.30.0.zip",
        ],
    )

    http_archive(
        name = "build_bazel_rules_nodejs",
        sha256 = "f7037c8e295fdc921f714962aee7c496110052511e2b14076bd8e2d46bc9819c",
        urls = ["https://github.com/bazelbuild/rules_nodejs/releases/download/4.4.5/rules_nodejs-4.4.5.tar.gz"],
    )

    http_archive(
        name = "bazel_gazelle",
        sha256 = "de69a09dc70417580aabf20a28619bb3ef60d038470c7cf8442fafcf627c21cb",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.24.0/bazel-gazelle-v0.24.0.tar.gz",
            "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.24.0/bazel-gazelle-v0.24.0.tar.gz",
        ],
    )

    http_archive(
        name = "rules_proto",
        sha256 = "66bfdf8782796239d3875d37e7de19b1d94301e8972b3cbd2446b332429b4df1",
        strip_prefix = "rules_proto-4.0.0",
        urls = [
            "https://github.com/bazelbuild/rules_proto/archive/refs/tags/4.0.0.tar.gz",
        ],
    )

    http_archive(
        name = "bats_support",
        url = "https://github.com/bats-core/bats-support/archive/refs/tags/v0.3.0.tar.gz",
        strip_prefix = "bats-support-0.3.0",
        build_file = "@enkit//bazel/dependencies:BUILD.bats_support.bazel",
        sha256 = "7815237aafeb42ddcc1b8c698fc5808026d33317d8701d5ec2396e9634e2918f",
    )

    http_archive(
        name = "bats_assert",
        url = "https://github.com/bats-core/bats-assert/archive/refs/tags/v2.0.0.tar.gz",
        strip_prefix = "bats-assert-2.0.0",
        build_file = "@enkit//bazel/dependencies:BUILD.bats_assert.bazel",
        sha256 = "15dbf1abb98db785323b9327c86ee2b3114541fe5aa150c410a1632ec06d9903",
    )

    http_archive(
        name = "bats_file",
        url = "https://github.com/bats-core/bats-file/archive/refs/tags/v0.2.0.tar.gz",
        strip_prefix = "bats-file-0.2.0",
        build_file = "@enkit//bazel/dependencies:BUILD.bats_file.bazel",
        sha256 = "1fa26407a68f4517cf9150d4763779ee66946a68eded33fa182ddf6a795c5062",
    )

    http_archive(
        name = "bats_core",
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

    http_archive(
        name = "rules_pkg",
        urls = [
            "https://github.com/bazelbuild/rules_pkg/releases/download/0.6.0/rules_pkg-0.6.0.tar.gz",
            "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.6.0/rules_pkg-0.6.0.tar.gz",
        ],
        sha256 = "62eeb544ff1ef41d786e329e1536c1d541bb9bcad27ae984d57f18f314018e66",
    )

    http_archive(
        name = "com_github_atlassian_bazel_tools",
        strip_prefix = "bazel-tools-5c3b9306e703c6669a6ce064dd6dde69f69cba35",
        sha256 = "c8630527150f3a9594e557fdcf02694e73420c10811eb214b461e84cb74c3aa8",
        urls = [
            "https://github.com/atlassian/bazel-tools/archive/5c3b9306e703c6669a6ce064dd6dde69f69cba35.zip",
        ],
    )

    http_archive(
        name = "gtest",
        sha256 = "94c634d499558a76fa649edb13721dce6e98fb1e7018dfaeba3cd7a083945e91",
        strip_prefix = "googletest-release-1.10.0",
        url = "https://github.com/google/googletest/archive/release-1.10.0.zip",
    )

    http_archive(
        name = "com_google_absl",
        urls = ["https://github.com/abseil/abseil-cpp/archive/98eb410c93ad059f9bba1bf43f5bb916fc92a5ea.zip"],
        strip_prefix = "abseil-cpp-98eb410c93ad059f9bba1bf43f5bb916fc92a5ea",
        sha256 = "aabf6c57e3834f8dc3873a927f37eaf69975d4b28117fc7427dfb1c661542a87",
    )

    http_archive(
        name = "bazel_skylib",
        sha256 = "af87959afe497dc8dfd4c6cb66e1279cb98ccc84284619ebfec27d9c09a903de",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.2.0/bazel-skylib-1.2.0.tar.gz",
            "https://github.com/bazelbuild/bazel-skylib/releases/download/1.2.0/bazel-skylib-1.2.0.tar.gz",
        ],
    )

    http_archive(
        name = "rules_python",
        sha256 = "cdf6b84084aad8f10bf20b46b77cb48d83c319ebe6458a18e9d2cebf57807cdd",
        strip_prefix = "rules_python-0.8.1",
        urls = [
            "https://github.com/bazelbuild/rules_python/archive/refs/tags/0.8.1.tar.gz",
            "https://mirror.bazel.build/github.com/bazelbuild/rules_python/archive/refs/tags/0.8.1.tar.gz",
        ],
    )

    http_archive(
        name = "com_github_grpc_grpc",
        patch_args = ["-p1"],
        patches = [
            "//bazel/dependencies/grpc:no_remote_tag.patch",
        ],
        sha256 = "12a4a6f8c06b96e38f8576ded76d0b79bce13efd7560ed22134c2f433bc496ad",
        strip_prefix = "grpc-1.41.1",
        urls = [
            "https://github.com/grpc/grpc/archive/refs/tags/v1.41.1.tar.gz",
        ],
    )

    http_archive(
        name = "com_github_bazelbuild_buildtools",
        strip_prefix = "buildtools-master",
        url = "https://github.com/bazelbuild/buildtools/archive/master.zip",
    )

    http_archive(
        name = "io_bazel_rules_docker",
        sha256 = "85ffff62a4c22a74dbd98d05da6cf40f497344b3dbf1e1ab0a37ab2a1a6ca014",
        strip_prefix = "rules_docker-0.23.0",
        urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.23.0/rules_docker-v0.23.0.tar.gz"],
    )

    http_archive(
        name = "rules_proto_grpc",
        sha256 = "28724736b7ff49a48cb4b2b8cfa373f89edfcb9e8e492a8d5ab60aa3459314c8",
        strip_prefix = "rules_proto_grpc-4.0.1",
        urls = ["https://github.com/rules-proto-grpc/rules_proto_grpc/archive/4.0.1.tar.gz"],
    )

    http_archive(
        name = "com_google_googleapis",
        urls = ["https://github.com/googleapis/googleapis/archive/10c88bb5c489c8ad1edb0e7f6a17cdd07147966e.zip"],
        strip_prefix = "googleapis-10c88bb5c489c8ad1edb0e7f6a17cdd07147966e",
        sha256 = "e8b434794608a9af0c0721cfaeedebe37d3676a4ee9dbeed868e5e2982b5abcc",
    )
