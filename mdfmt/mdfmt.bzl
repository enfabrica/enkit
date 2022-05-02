"""mdfmt file filter rule."""

def _mdfmt_filter_impl(ctx):
    args = ctx.actions.args()

    for f in ctx.files.src:
        args.add(f)
    args.add("--output", ctx.outputs.out.path)

    ctx.actions.run(
        inputs = ctx.files.src,
        outputs = [ctx.outputs.out],
        executable = ctx.executable.mdfmt_tool,
        arguments = [args],
    )

mdfmt_filter = rule(
    doc = """
      Filters a markdown file through mdfmt.

      This is for use in series with code generating rules that
      produce unformatted markdown.
    """,
    implementation = _mdfmt_filter_impl,
    output_to_genfiles = True,  # so that header files can be found.
    attrs = {
        "src": attr.label(
            allow_files = [".md", ".md.unformatted"],
            doc = "Markdown file to process.",
        ),
        "out": attr.output(
            mandatory = True,
            doc = "Formatted markdown file to generate.",
        ),
        "mdfmt_tool": attr.label(
            executable = True,
            cfg = "exec",
            allow_files = True,
            default = Label("//mdfmt:mdfmt"),
            doc = "The path to the mdfmt tool itself.",
        ),
    },
)
