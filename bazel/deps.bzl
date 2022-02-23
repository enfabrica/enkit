load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

def enkit_deps():
    excludes = native.existing_rules().keys()

    if "io_bazel_rules_go" not in excludes:
        http_archive(
            name = "io_bazel_rules_go",
            sha256 = "2b1641428dff9018f9e85c0384f03ec6c10660d935b750e3fa1492a281a53b0f",
            urls = [
                "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
                "https://github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
            ],
        )

    if "build_bazel_rules_nodejs" not in excludes:
        http_archive(
            name = "build_bazel_rules_nodejs",
            sha256 = "f7037c8e295fdc921f714962aee7c496110052511e2b14076bd8e2d46bc9819c",
            urls = ["https://github.com/bazelbuild/rules_nodejs/releases/download/4.4.5/rules_nodejs-4.4.5.tar.gz"],
        )

    if "bazel_gazelle" not in excludes:
        http_archive(
            name = "bazel_gazelle",
            sha256 = "de69a09dc70417580aabf20a28619bb3ef60d038470c7cf8442fafcf627c21cb",
            urls = [
                "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.24.0/bazel-gazelle-v0.24.0.tar.gz",
                "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.24.0/bazel-gazelle-v0.24.0.tar.gz",
            ],
        )

    if "rules_proto" not in excludes:
        http_archive(
            name = "rules_proto",
            sha256 = "66bfdf8782796239d3875d37e7de19b1d94301e8972b3cbd2446b332429b4df1",
            strip_prefix = "rules_proto-4.0.0",
            urls = [
                "https://github.com/bazelbuild/rules_proto/archive/refs/tags/4.0.0.tar.gz",
            ],
        )

    if "bats_core" not in excludes:
        # bats: Bash Automated Testing System
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

    # rules_docker 0.14.4 is incompatible with rules_pkg 0.3.0 as of Oct/2020.
    #
    # When you update this dependency, please make sure rules_docker has been updated as well,
    # and do run a docker build to ensure that there is no breakage.
    if "rules_pkg" not in excludes:
        http_archive(
            name = "rules_pkg",
            urls = [
                "https://github.com/bazelbuild/rules_pkg/releases/download/0.2.6-1/rules_pkg-0.2.6.tar.gz",
                "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.2.6/rules_pkg-0.2.6.tar.gz",
            ],
            sha256 = "aeca78988341a2ee1ba097641056d168320ecc51372ef7ff8e64b139516a4937",
        )

    if "com_github_atlassian_bazel_tools" not in excludes:
        http_archive(
            name = "com_github_atlassian_bazel_tools",
            strip_prefix = "bazel-tools-5c3b9306e703c6669a6ce064dd6dde69f69cba35",
            sha256 = "c8630527150f3a9594e557fdcf02694e73420c10811eb214b461e84cb74c3aa8",
            urls = [
                "https://github.com/atlassian/bazel-tools/archive/5c3b9306e703c6669a6ce064dd6dde69f69cba35.zip",
            ],
        )

    if "gtest" not in excludes:
        http_archive(
            name = "gtest",
            sha256 = "94c634d499558a76fa649edb13721dce6e98fb1e7018dfaeba3cd7a083945e91",
            strip_prefix = "googletest-release-1.10.0",
            url = "https://github.com/google/googletest/archive/release-1.10.0.zip",
        )

    if "com_google_absl" not in excludes:
        http_archive(
            name = "com_google_absl",
            urls = ["https://github.com/abseil/abseil-cpp/archive/98eb410c93ad059f9bba1bf43f5bb916fc92a5ea.zip"],
            strip_prefix = "abseil-cpp-98eb410c93ad059f9bba1bf43f5bb916fc92a5ea",
            sha256 = "aabf6c57e3834f8dc3873a927f37eaf69975d4b28117fc7427dfb1c661542a87",
        )
