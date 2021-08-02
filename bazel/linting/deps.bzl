load("//bazel/utils:binary.bzl", "download_binary")
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

def lint_deps_init():
    http_archive(
        name = "golangci_lint_linux_amd64",
        urls = [
            "https://github.com/golangci/golangci-lint/releases/download/v1.41.1/golangci-lint-1.41.1-linux-amd64.tar.gz",
        ],
        build_file_content = 'exports_files(glob(["**/*"]))',
        sha256 = "23e1078ab00a750afcde7e7eb5aab8e908ef18bee5486eeaa2d52ee57d178580",
        strip_prefix = "golangci-lint-1.41.1-linux-amd64",
    )
