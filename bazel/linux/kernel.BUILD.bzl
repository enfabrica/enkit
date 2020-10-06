filegroup(
    name = "{name}-tree",
    srcs = glob(["**/*"]),
    visibility = [
        "//visibility:public",
    ],
)

load("{utils}", "kernel_tree")

kernel_tree(
    name = "{name}",
    package = "{package}",
    files = [":{name}-tree"],
    build = "{build}",
    visibility = [
        "//visibility:public",
    ],
)
