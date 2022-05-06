load("{utils}", "rootfs_image")

rootfs_image(
    name = "{name}",
    image = "{image}",
    visibility = [
        "//visibility:public",
    ],
)
