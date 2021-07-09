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

    if "com_google_protobuf" not in excludes:
        http_archive(
	    name = "com_google_protobuf",
	    sha256 = "c6003e1d2e7fefa78a3039f19f383b4f3a61e81be8c19356f85b6461998ad3db",
	    strip_prefix = "protobuf-3.17.3",
	    urls = [
                "https://github.com/protocolbuffers/protobuf/archive/v3.17.3.tar.gz",
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

    if "bazel_skylib" not in excludes:
        http_archive(
            name = "bazel_skylib",
            urls = [
                "https://github.com/bazelbuild/bazel-skylib/releases/download/1.0.3/bazel-skylib-1.0.3.tar.gz",
                "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.0.3/bazel-skylib-1.0.3.tar.gz",
            ],
            sha256 = "1c531376ac7e5a180e0237938a2536de0c54d93f5c278634818e0efc952dd56c",
        )
