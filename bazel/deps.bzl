load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

def enkit_deps():
    excludes = native.existing_rules().keys()

    if "io_bazel_rules_go" not in excludes:
        http_archive(
            name = "io_bazel_rules_go",
            sha256 = "69de5c704a05ff37862f7e0f5534d4f479418afc21806c887db544a316f3cb6b",
            urls = [
                "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.27.0/rules_go-v0.27.0.tar.gz",
                "https://github.com/bazelbuild/rules_go/releases/download/v0.27.0/rules_go-v0.27.0.tar.gz",
            ],
        )

    if "com_github_ccontavalli_bazel_rules" not in excludes:
        http_archive(
            name = "com_github_ccontavalli_bazel_rules",
            sha256 = "0d0d8e644fd616d0ee225444889295914405df77cc549e8fc87ad6fd8b9bbb25",
            strip_prefix = "bazel-rules-6",
            urls = ["https://github.com/ccontavalli/bazel-rules/archive/v6.tar.gz"],
        )

    if "build_bazel_rules_nodejs" not in excludes:
        http_archive(
            name = "build_bazel_rules_nodejs",
            sha256 = "4a5d654a4ccd4a4c24eca5d319d85a88a650edf119601550c95bf400c8cc897e",
            urls = ["https://github.com/bazelbuild/rules_nodejs/releases/download/3.5.1/rules_nodejs-3.5.1.tar.gz"],
        )

    if "bazel_gazelle" not in excludes:
        http_archive(
            name = "bazel_gazelle",
            sha256 = "62ca106be173579c0a167deb23358fdfe71ffa1e4cfdddf5582af26520f1c66f",
            urls = [
                "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
                "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
            ],
        )

    if "rules_proto" not in excludes:
        http_archive(
            name = "rules_proto",
            sha256 = "aa1ee19226f707d44bee44c720915199c20c84a23318bb0597ed4e5c873ccbd5",
            strip_prefix = "rules_proto-40298556293ae502c66579620a7ce867d5f57311",
            urls = [
                "https://mirror.bazel.build/github.com/bazelbuild/rules_proto/archive/40298556293ae502c66579620a7ce867d5f57311.tar.gz",
                "https://github.com/bazelbuild/rules_proto/archive/40298556293ae502c66579620a7ce867d5f57311.tar.gz",
            ],
        )

    if "bats_core" not in excludes:
        # bats: Bash Automated Testing System
        http_archive(
            name = "bats_support",
            url = "https://github.com/bats-core/bats-support/archive/refs/tags/v0.3.0.tar.gz",
            strip_prefix="bats-support-0.3.0",
            build_file = "@//bazel/dependencies:BUILD.bats_support.bazel",
            sha256 = "7815237aafeb42ddcc1b8c698fc5808026d33317d8701d5ec2396e9634e2918f",
        )

        http_archive(
            name = "bats_assert",
            url = "https://github.com/bats-core/bats-assert/archive/refs/tags/v2.0.0.tar.gz",
            strip_prefix="bats-assert-2.0.0",
            build_file = "@//bazel/dependencies:BUILD.bats_assert.bazel",
            sha256 = "15dbf1abb98db785323b9327c86ee2b3114541fe5aa150c410a1632ec06d9903",
        )

        http_archive(
            name = "bats_core",
            url = "https://github.com/bats-core/bats-core/archive/refs/tags/v1.4.1.tar.gz",
            strip_prefix="bats-core-1.4.1",
            build_file = "@//bazel/dependencies:BUILD.bats.bazel",
            sha256 = "bff517da043ae24440ec8272039f396c2a7907076ac67693c0f18d4a17c08f7d",
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
            strip_prefix = "bazel-tools-8b69172a66e62060a628e13111ca8d9072c4978e",
            sha256 = "58aa5457f743642e77076c817f8c62403d0b1c9b610051a1a459e3478bb92a61",
            urls = [
                "https://github.com/atlassian/bazel-tools/archive/8b69172a66e62060a628e13111ca8d9072c4978e.zip",
            ],
        )
