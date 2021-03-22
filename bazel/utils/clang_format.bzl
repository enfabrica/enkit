def _clang_format(ctx):
    style_file = ctx.file.style_file
    if ctx.attr.style == "file" and style_file == None:
        fail("style=file but no style_file specified.")

    clang_format = ctx.file.clang_format
    out_file = ctx.actions.declare_file(ctx.label.name + ".bash")
    substitutions = {
        "{pattern}": ctx.attr.pattern,
        "{clang-format}": clang_format.short_path,
        "{style}": ctx.attr.style,
        "{style-file}": style_file.short_path,
    }
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = out_file,
        substitutions = substitutions,
        is_executable = True,
    )

    return DefaultInfo(
        files = depset([out_file]),
        runfiles = ctx.runfiles(files = [clang_format, style_file]),
        executable = out_file,
    )

clang_format = rule(
    doc = """Formats all the files matching a pattern using the specified style.

This rule formats in-place all the files belonging to the Bazel workspace that
match the specified pattern.
It depends on `clang-format` and it expects to retrieve it from the clang_format
label parameter.

As an example, you can use:
[WORSPACE]
    http_file(
        name = "clang-format",
        urls = [
            "https://address_to_download_clang-format_from",
        ],
        executable = True,
    )

[BUILD.bazel]
    clang_format(
        name = "cc_format",
        pattern = "*.cc",
        style = "Google",
        clang_format = "@clang-format//file",
    )
    clang_format(
        name = "cc_format",
        pattern = "*.c",
        style = "file",
        style_file = "c.clang-format",
        clang_format = "@clang-format//file",
    )

To download clang-format from "https://address_to_donwload_clang-format_from" and use it
to: format all .cc files in the workspace accordingly to the Google style guide, and all
.c files accordingly to the style specified in the c.clang-format file.
""",
    implementation = _clang_format,
    attrs = {
        "style": attr.string(
            mandatory = True,
            # From clang-format-7 --help output.
            values = ["LLVM", "Google", "Chromium", "Mozilla", "WebKit", "file"],
            doc = "The coding style to apply. If equal to 'file' the file_style parameter is required.",
        ),
        # TODO: Support a list of patterns to format.
        "pattern": attr.string(
            mandatory = True,
            doc = "The pattern that the files to format must match.",
        ),
        "style_file": attr.label(
            allow_single_file = True,
            doc = ".clang-format file to use in case style=file.",
        ),
        "clang_format": attr.label(
            allow_single_file = True,
            mandatory = True,
            doc = "Label of the clang-format executable to use.",
        ),
        "_template": attr.label(
            allow_single_file = True,
            default = Label("//bazel/utils:run_clang_format.template.sh"),
            doc = "The template used to generate the bash script that will run clang-format.",
        ),
    },
    executable = True,
)
