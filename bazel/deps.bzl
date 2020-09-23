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
            sha256 = "d14076339deb08e5460c221fae5c5e9605d2ef4848eee1f0c81c9ffdc1ab31c1",
            urls = ["https://github.com/bazelbuild/rules_nodejs/releases/download/1.6.1/rules_nodejs-1.6.1.tar.gz"],
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
