load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

def enkit_deps():
    excludes = native.existing_rules().keys()

    if "io_bazel_rules_go" not in excludes:
        http_archive(
            name = "io_bazel_rules_go",
            sha256 = "b725e6497741d7fc2d55fcc29a276627d10e43fa5d0bb692692890ae30d98d00",
            urls = [
                "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.24.3/rules_go-v0.24.3.tar.gz",
                "https://github.com/bazelbuild/rules_go/releases/download/v0.24.3/rules_go-v0.24.3.tar.gz",
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
            sha256 = "10f534e1c80f795cffe1f2822becd4897754d18564612510c59b3c73544ae7c6",
            urls = ["https://github.com/bazelbuild/rules_nodejs/releases/download/3.5.0/rules_nodejs-3.5.0.tar.gz"],
        )

    if "bazel_gazelle" not in excludes:
        http_archive(
            name = "bazel_gazelle",
            sha256 = "72d339ff874a382f819aaea80669be049069f502d6c726a07759fdca99653c48",
            urls = [
                "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.22.1/bazel-gazelle-v0.22.1.tar.gz",
                "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.22.1/bazel-gazelle-v0.22.1.tar.gz",
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
