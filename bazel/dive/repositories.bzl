load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:utils.bzl", "maybe")

def dive_dependencies():
    maybe(
        http_archive,
        name = "dive_x86_64",
        build_file_content = """
filegroup(
    name = "dive_x86_64",
    srcs = ["dive"],
    visibility = ["//visibility:public"],
)
""",
        sha256 = "20a7966523a0905f950c4fbf26471734420d6788cfffcd4a8c4bc972fded3e96",
        url = "https://github.com/wagoodman/dive/releases/download/v0.12.0/dive_0.12.0_linux_amd64.tar.gz",
    )

