load("@enkit//bazel/linux:defs.bzl", "kernel_image")

kernel_image(
    name = "image",
    package = "{version}",
    arch = "host",
    image = "boot/vmlinuz-{version}",
    visibility = [
        "//visibility:public",
    ],
)

filegroup(
    name = "modules",
    srcs = glob(["lib/modules/**"],
    exclude = [
        "**/add-ons/**",
    ]),
    visibility = [
        "//visibility:public",
    ],
)
