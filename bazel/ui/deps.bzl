load("//bazel/utils:binary.bzl", "download_binary")

def install_ui_deps():
    download_binary(
        name = "jq_linux_amd64",
        binary_name = "jq",
        uri = "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64",
    )
    download_binary(
        name = "jq_macos_amd64",
        binary_name = "jq",
        uri = "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-osx-amd64",
    )
