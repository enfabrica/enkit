"""Template bazel rule definitions

Forked from: https://github.com/ccontavalli/bazel-rules/blob/master/qtc/defs.bzl
"""

load("@rules_go//go:def.bzl", "go_library")

def _qtpl_compile(ctx):
    outputs = []
    for src in ctx.files.srcs:
        out = ctx.actions.declare_file(src.path + ".go")
        ctx.actions.run_shell(
            inputs = [src],
            tools = [ctx.executable._compiler],
            outputs = [out],
            mnemonic = "QTPLCompile",
            progress_message = "Compiling template %s" % out.short_path,
            command = "%s -file %s && mv %s.go %s" % (ctx.executable._compiler.path, src.path, src.path, out.path),
        )
        outputs.append(out)

    return [DefaultInfo(files = depset(outputs))]

qtpl_compile = rule(
    implementation = _qtpl_compile,
    attrs = {
        "srcs": attr.label_list(
            allow_files = True,
            mandatory = True,
            doc = "qtpl template files to compile into go",
        ),
        "_compiler": attr.label(
            executable = True,
            default = Label("@com_github_valyala_quicktemplate//qtc"),
            cfg = "exec",
        ),
    },
    doc = """
Creates a golang library from the specified .qtpl files.
""",
)

def qtpl_go_library(name, srcs, **kwargs):
    qtpl_compile(name = name + "-qtpl", srcs = srcs)
    kwargs.setdefault("deps", []).append("@com_github_valyala_quicktemplate//:go_default_library")
    return go_library(name = name, srcs = [":" + name + "-qtpl"], **kwargs)
