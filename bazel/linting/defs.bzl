load("@io_bazel_rules_go//go:def.bzl", "go_context")



def _generate_list_of_changes_files(ctx):
    ctx.actions.run(
        executable = ctx.executable._to_execute,
#        input_files = ctx.attr._root_repository.files,
        outputs = [ctx.outputs.out]
    )


get_changed_files = rule(
    _generate_list_of_changes_files,
    attrs = {
        "_to_execute": attr.label(
            default = Label("//bazel/linting/scripts:git.sh"),
            allow_single_file = True,
            executable = True,
            cfg = "exec"
        ),
#        "_root_repository": attr.label(
#            default = Label("//.git")
#        )
    },
    outputs = {
        "out": "hello"
    }
)

def generate_git_changes(name, visibility=None):
    native.genrule(
        name = 'git_version',
        srcs = ['.git/HEAD', '.git/refs/**'],
        outs = [
          'changes.txt',
        ],
        cmd = 'echo "$$(git rev-parse HEAD)" > $(location changes.txt)',
    )


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
            default = Label("//bazel/linting/scripts:lint.sh"),
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


def lint(name, go_libs, rust_libs):
    native.genrule(
       name = 'git_version',
       srcs = native.glob(['.git/HEAD', '.git/refs/**']),
       outs = [
         'changes.txt',
       ],
       cmd = "echo $$(git rev-parse HEAD) > $@",
#       cmd = "echo $$(git rev-parse HEAD) > $@",
    )
    print("done with the git command")
