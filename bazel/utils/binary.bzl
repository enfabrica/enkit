load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_file")

def _download_binary(ctx):
    ctx.download(
        url = [
            ctx.attr.uri,
        ],
        output = ctx.attr.binary_name,
        executable = True,
    )
    ctx.file(
        "BUILD.bazel",
        content = 'exports_files(glob(["*"]))',
    )

download_binary = repository_rule(
    _download_binary,
    doc = """Downloads a single binary that is not tarballed.

               Example:
                     download_binary(
                           name = "jq_macos_amd64",
                           binary_name = "jq",
                           uri = "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-osx-amd64",
                       )
               """,
    attrs = {
        "binary_name": attr.string(
            mandatory = True,
        ),
        "uri": attr.string(
            mandatory = True,
        ),
    },
)

def _declare_binary(ctx):
    out = ctx.actions.declare_file(ctx.attr.binary_name)
    ctx.actions.symlink(
        output = out,
        target_file = ctx.files.binary[0],
    )
    return DefaultInfo(executable = out)

declare_binary = rule(
    _declare_binary,
    doc = """Declares a single binary, used as a wrapper around a select() statement

              Example:
                  declare_binary(
                      name = "jq",
                      binary = select({
                          "@platforms//os:linux": "@jq_linux_amd64//:jq",
                          "@platforms//os:osx": "@jq_macos_amd64//:jq",
                      }),
                      binary_name = "jq",
                      visibility = ["//visibility:public"],
                  )

              """,
    attrs = {
        "binary_name": attr.string(
            mandatory = True,
        ),
        "binary": attr.label(
            mandatory = True,
            executable = True,
            cfg = "exec",
            allow_files = True,
        ),
    },
)
