def _clang_format(ctx):
    clang_format_file = ctx.file.clang_format
    out_file = ctx.actions.declare_file(ctx.label.name + ".bash")
    substitutions = {
        "{pattern-to-format}": ctx.attr.pattern_to_format,
        "{clang-format}": clang_format_file.short_path,
        "{style}": ctx.attr.style,
    }
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = out_file,
        substitutions = substitutions,
        is_executable = True,
    )

    return DefaultInfo(
        files = depset([out_file]),
        runfiles = ctx.runfiles(files = [clang_format_file]),
        executable = out_file,
    )

clang_format = rule(
    implementation = _clang_format,
    attrs = {
        "style": attr.string(
            mandatory = True,
            # From clang-format-7 --help output.
            values = ["LLVM", "Google", "Chromium", "Mozilla", "WebKit", "file"],
            doc = "The coding style to apply.",
        ),
        # TODO: Support a list of patterns to format.
        "pattern_to_format": attr.string(
            mandatory = True,
            doc = "The name pattern of the files to format.",
        ),
        "clang_format": attr.label(
            allow_single_file = True,
            mandatory = True,
            doc = "Label of the clang-format executable to use.",
        ),
        "_template": attr.label(
            allow_single_file = True,
            default = Label("//bazel/utils:run_clang_format.template"),
            doc = "The template used to generate the bash script that will run clang-format.",
        ),
    },
    executable = True,
)
