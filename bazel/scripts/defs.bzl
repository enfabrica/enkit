load("@io_bazel_rules_go//go:def.bzl", "go_context")

# TODO be able to put in a list of go_path targets to lint
# TODO make flatmap helper?
# TODO export lint result file
def _my_rule_impl(ctx):
    lib = ctx.attr.go_library[0]
    library_name = lib.label.name
    go = go_context(ctx)
    print(lib)
    print(ctx.outputs.output.path)
    ctx.actions.run(
        inputs = lib.files,
        outputs = [ctx.outputs.output],
        arguments = [],
        progress_message = "Running linter into",
        executable = ctx.executable._lint_script,
        tools = [go.go],
        env = {
            "GO_LOCATION": go.sdk.root_file.dirname,
            "GO_LIBRARY_NAME": library_name,
            "LINT_OUTPUT": ctx.outputs.output.path
        }
    )
    print("ran all the stuff")



go_lint = rule(
    _my_rule_impl,
    attrs = {
        "go_library": attr.label_list(),
        "output": attr.output(mandatory = True),
        "_lint_script": attr.label(
            default = Label("//bazel/scripts:lint.sh"),
            allow_files = True,
            executable = True,
            cfg = "exec"
        ),
         "_go_context_data": attr.label(
            default = "@io_bazel_rules_go//:go_context_data",
        ),
    },
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)

