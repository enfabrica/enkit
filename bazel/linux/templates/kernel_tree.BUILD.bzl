load("{utils}", "kernel_tree")

filegroup(
    name = "{name}-tree",
    # Why allow_empty = True? To support compatibility with different package formats,
    # they may expand the content in different directories, and not use others.
    srcs = glob(["lib", "usr", "install", "source"], allow_empty = True, exclude_directories = 0),
    visibility = [
        "//visibility:public",
    ],
)

kernel_tree(
    name = "{name}",
    package = "{package}",
    arch = "{arch}",
    files = [":{name}-tree"],
    build = "{build_path}",
    visibility = [
        "//visibility:public",
    ],
)
