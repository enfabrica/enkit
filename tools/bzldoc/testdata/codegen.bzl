"""Test bzl file for bzldoc.

This is a phoney baloney bzl file for testing bzldoc.

It is a fork of codegen.bzl.
"""

load("//bazel/utils:diff_test.bzl", "diff_test")


def _codegen_impl(ctx):
    args = ctx.actions.args()
    for f in ctx.files.schema:
        args.add("--schema", f)
    for f in ctx.files.data:
        args.add("--load", f)
    for f in ctx.files.srcs:
        args.add(f)
    if ctx.attr.multigen_mode:
        args.add("--multigen_mode")
    for f in ctx.outputs.outs:
        args.add("--output", f.path)
    for kvpair in ctx.attr.overrides:
        args.add("--override", kvpair)

    ctx.actions.run(
        inputs = ctx.files.data + ctx.files.srcs + ctx.files.schema,
        outputs = ctx.outputs.outs,
        executable = ctx.executable.codegen_tool,
        arguments = [args],
        progress_message = "Generating %s" % ",".join([repr(x.short_path) for x in ctx.outputs.outs]),
    )

codegen = rule(
    doc = """
      Runs codegen to combine templates and data files to an artifact.
    """,
    implementation = _codegen_impl,
    output_to_genfiles = True,  # so that header files can be found.
    attrs = {
        "data": attr.label_list(
            allow_files = [".json", ".yaml"],
            doc = "An ordered list of data files to load.",
        ),
        "outs": attr.output_list(
            allow_empty = False,
            doc = "Artifacts to generate.",
        ),
        "srcs": attr.label_list(
            allow_files = [".jinja2", ".jinja", ".template"],
            doc = "A list of jinja2 template files to import.",
        ),
        "schema": attr.label(
            allow_files = [".schema", "schema.yaml"],
            doc = "A jsonschema file to check the imported data against.",
        ),
        "overrides": attr.string_list(doc = "A pair of key=value pairs to override context data."),
        "template_name": attr.string(doc = "The specific jinja2 template to render (optional)."),
        "multigen_mode": attr.bool(doc = "Enable multigen mode."),
        "codegen_tool": attr.label(
            executable = True,
            cfg = "exec",
            allow_files = True,
            default = Label("//tools/codegen:codegen"),
            doc = "The path to the codegen tool itself.",
        ),
    },
)

def codegen_test(name, expected = None, **codegen_args):
    codegen(
        name = name + "-actual-gen",
        outs = [name + ".actual"],
        **codegen_args
    )
    if not expected:
        expected = name + ".expected"
    diff_test(
        name = name,
        actual = name + "-actual-gen",
        expected = expected,
    )
