load("@io_bazel_rules_go//go:def.bzl", "go_context", "go_path")


DepInfo = provider(
    fields = {
        'files' : 'deps'
    }
)

def _dep_aspect_impl(target, ctx):
    # Make sure the rule has a srcs attribute.
    to_return = []
    if hasattr(ctx.rule.attr, 'srcs'):
        for src in ctx.rule.attr.srcs:
            for f in src.files.to_list():
                to_return.append(f)

    if hasattr(ctx.rule.attr, 'deps'):
        for dep in ctx.rule.attr.deps:
            to_return.extend(dep[DepInfo].files)

    return [DepInfo(files=to_return)]

dep_aspect = aspect(
    implementation = _dep_aspect_impl,
    attr_aspects = ['deps'],
)

def _go_lint_impl(ctx):
    lib = ctx.attr.go_libraries[0]
    library_name = lib.label.name
    go = go_context(ctx)
    inputs = []
    for l in ctx.attr.deps:
        inputs.extend(l[DefaultInfo].files.to_list())

    for lib in ctx.attr.go_libraries:
        inputs.extend(lib.files.to_list())

    ctx.actions.run(
        inputs = depset(inputs),
        outputs = [ctx.outputs.out],
        arguments = [],
        progress_message = "Running linter into",
        executable = ctx.executable._lint_script,
        tools = [go.go],
        env = {
            "GO_LOCATION": go.sdk.root_file.dirname,
            "GO_LIBRARY_NAME": library_name,
            "LINT_OUTPUT": ctx.outputs.out.path,
            "GIT_DATA": inputs[0].path
        }
    )



go_lint = rule(
    _go_lint_impl,
    attrs = {
        "go_libraries": attr.label_list(
            aspects = [dep_aspect]
        ),
        "_lint_script": attr.label(
            default = Label("//bazel/linting/scripts:lint_go.sh"),
            allow_files = True,
            executable = True,
            cfg = "exec"
        ),
         "_go_context_data": attr.label(
            default = "@io_bazel_rules_go//:go_context_data",
        ),
        "deps": attr.label_list(),
    },
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
    outputs = {
        "out": "go_lint_result.txt"
    }
)

#go_libs takes in a gopath compliant repo.
def lint(name, go_libs, rust_libs):
    native.genrule(
        name = "parse_git_changes",
        outs = [
            "//:git.txt"
        ],
        srcs = [
            "//bazel/linting/scripts:git.sh"
        ],
        cmd = "./$(location //bazel/linting/scripts:git.sh) bazel-out/volatile-status.txt > $@",
        stamp = 1
    )
    go_path_rules = []
    go_lint_ouputs = []
    for go_lib in go_libs:
        short_name = go_lib.replace("/","_").replace(":", "_")
        go_path(name = short_name + "_source",
                    deps = [go_lib],
                    mode = "copy")
        go_path_rules.append("//:" + short_name + "_source")
        go_lint_ouputs.append(short_name + ".txt")

    go_lint(
        name="lint_go",
        go_libraries=go_path_rules,
        deps = [
            ":parse_git_changes",
        ]
    )
