load("{utils}", "kernel_image")

kernel_image(
    name = "{name}",
    package = "{package}",
    arch = "{arch}",
    image = "{image_path}",
    visibility = [
        "//visibility:public",
    ],
)
