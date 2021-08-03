load("@io_bazel_rules_go//go:def.bzl", "go_context", "go_path")
load("//bazel/utils:binary.bzl", "declare_binary", "download_binary")

DepInfo = provider(
    fields = {
        "files": "deps",
    },
)

def _dep_aspect_impl(target, ctx):
    # Make sure the rule has a srcs attribute.
    to_return = []
    if hasattr(ctx.rule.attr, "srcs"):
        for src in ctx.rule.attr.srcs:
            for f in src.files.to_list():
                to_return.append(f)

    if hasattr(ctx.rule.attr, "deps"):
        for dep in ctx.rule.attr.deps:
            to_return.extend(dep[DepInfo].files)

    return [DepInfo(files = to_return)]

dep_aspect = aspect(
    implementation = _dep_aspect_impl,
    attr_aspects = ["deps"],
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

    outfiles = []
    for target in ctx.attr.targets:
        outfile = ctx.actions.declare_file(target + ".txt")
        outfiles.append(outfile)
        ctx.actions.run(
            inputs = depset(inputs),
            outputs = [outfile],
            progress_message = "Running linter into",
            executable = ctx.files._lint_script[0],
            tools = [go.go, ctx.files._golangci_lint[0]],
            env = {
                "GO_LOCATION": go.sdk.root_file.dirname,
                "GO_LIBRARY_NAME": library_name,
                "LINT_OUTPUT": outfile.path,
                "GIT_DATA": inputs[0].path,
                "GOLANGCI_LINT": ctx.files._golangci_lint[0].path,
                "TARGET": target,
            },
        )
    return DefaultInfo(
        files = depset(outfiles),
    )

go_lint = rule(
    _go_lint_impl,
    attrs = {
        "go_libraries": attr.label_list(
            aspects = [dep_aspect],
        ),
        "targets": attr.string_list(
            mandatory = True,
        ),
        "_lint_script": attr.label(
            default = Label("//bazel/linting/scripts:lint_go.sh"),
            allow_files = True,
            executable = True,
            cfg = "exec",
        ),
        "_go_context_data": attr.label(
            default = "@io_bazel_rules_go//:go_context_data",
        ),
        "_golangci_lint": attr.label(
            allow_files = True,
            default = "//bazel/linting/scripts:golangci-lint",
        ),
        "_merge_script": attr.label(
            allow_files = True,
            default = "//bazel/linting/scripts:merge.sh",
        ),
        "deps": attr.label_list(),
    },
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)

#go_libs takes in a gopath compliant repo.
def lint(go_libs):
    native.genrule(
        name = "parse_git_changes",
        outs = [
            "//:git.txt",
        ],
        srcs = [
            "//bazel/linting/scripts:git.sh",
        ],
        cmd = "./$(location //bazel/linting/scripts:git.sh) bazel-out/stable-status.txt > $@",
        stamp = 1,
    )
    go_path_rules = []
    go_lint_targets = []
    for go_lib in go_libs:
        short_name = go_lib.replace("/", "_").replace(":", "_")
        go_path(
            name = short_name + "_source",
            deps = [go_lib],
            mode = "copy",
        )
        go_path_rules.append("//:" + short_name + "_source")
        go_lint_targets.append(short_name)
    go_lint(
        name = "lint_go",
        go_libraries = go_path_rules,
        targets = go_lint_targets,
        deps = [
            ":parse_git_changes",
        ],
    )

def _check_lint(ctx):
    outfile = ctx.actions.declare_file("meow.txt")
    args = ctx.actions.args()
    args.add(outfile.path)
    for f in ctx.files.lint:
        args.add(f.path)

    ctx.actions.run(
        executable = ctx.files._lint_script[0],
        inputs = ctx.files.lint,
        outputs = [outfile],
        arguments = [args],
        progress_message = "Parsing lint files",
        tools = ctx.files._lint_script,
        env = {
            "OUT": outfile.path,
            "STRATEGY": ctx.attr.strategy,
            "GIT_FILE": "",
        },
    )
    return DefaultInfo(
        files = depset([outfile]),
    )

parse_lint = rule(
    _check_lint,
    attrs = {
        "lint": attr.label(
            mandatory = True,
        ),
        "_lint_script": attr.label(
            default = "//bazel/linting/scripts:merge.sh",
            allow_files = True,
        ),
        "strategy": attr.string(
            default = "ALL",
        ),
    },
)
