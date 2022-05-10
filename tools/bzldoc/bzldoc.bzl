load("//tools/codegen:codegen.bzl", "codegen")
load("//tools/mdfmt:mdfmt.bzl", "mdfmt_filter")

def _bzl2yaml_impl(ctx):
    args = ctx.actions.args()

    for f in ctx.files.src:
        args.add("--input", f)
        args.add("--short_path", f.short_path)
    args.add("--output", ctx.outputs.out.path)

    ctx.actions.run(
        inputs = ctx.files.src,
        outputs = [ctx.outputs.out],
        executable = ctx.executable.bzl2yaml_tool,
        arguments = [args],
    )

bzl2yaml = rule(
    doc = """
      Runs bzl2yaml to parse a bzl file into data.
    """,
    implementation = _bzl2yaml_impl,
    output_to_genfiles = True,  # so that header files can be found.
    attrs = {
        "src": attr.label(
            allow_files = [".bzl"],
            doc = "BZL file to parse.",
        ),
        "out": attr.output(
            mandatory = True,
            doc = "YAML file to generate.",
        ),
        "bzl2yaml_tool": attr.label(
            executable = True,
            cfg = "exec",
            allow_files = True,
            default = Label("//tools/bzldoc:bzl2yaml"),
            doc = "The path to the bzl2yaml tool itself.",
        ),
    },
)

def bzldoc(name, src):
    """Convert a BZL file into documentation."""
    bzl2yaml(
        name = "%s-bzl2yaml" % name,
        src = src,
        out = "%s.yaml" % name,
    )
    codegen(
        name = "%s-md-unformatted-gen" % name,
        outs = [name + ".md.unformatted"],
        srcs = ["@enkit//tools/bzldoc:md.template"],
        data = ["%s.yaml" % name],
        visibility = ["//visibility:public"],
    )
    mdfmt_filter(
        name = "%s-md-gen" % name,
        out = name + ".md",
        src = name + ".md.unformatted",
        visibility = ["//visibility:public"],
    )
