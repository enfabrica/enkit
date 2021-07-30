filegroup(
    name = "{name}-tree",
    srcs = glob(["*"], allow_empty=False, exclude_directories=0),
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
