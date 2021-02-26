load("{utils}", "kernel_image")

kernel_image(
    name = "{name}",
    package = "{package}",
    image = "{image}",
    visibility = [
        "//visibility:public",
    ],
)
