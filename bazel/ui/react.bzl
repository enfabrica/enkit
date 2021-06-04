load("@npm//react-scripts:index.bzl", "react_scripts", "react_scripts_test")
load("@build_bazel_rules_nodejs//:index.bzl", "copy_to_bin", "nodejs_test")
load("@bazel_skylib//rules:write_file.bzl", "write_file")
load("@build_bazel_rules_nodejs//:index.bzl", "nodejs_binary")
load("@bazel_skylib//lib:paths.bzl", "paths")
load("//bazel/utils:transform.bzl", "transform")

# doing this until a better option comes along
def trim_path(path, prefix, base):
    s = path.split("/")
    tret = []
    flag = False
    for m in s:
        if m == prefix:
            flag = True
        if flag:
            tret.append(m)
    return paths.join(base, *tret)

def _copy_files_new_dir_impl(ctx):
    all_input_files = [
        f
        for t in ctx.attr.source_files
        for f in t.files.to_list()
    ]
    all_outputs = []
    for f in all_input_files:
        if ctx.attr.no_prefix:
            out_path = paths.join(ctx.attr.base_dir, f.basename)
        else:
            out_path = trim_path(f.short_path, ctx.attr.prefix, ctx.attr.base_dir)
        out = ctx.actions.declare_file(out_path)
        all_outputs += [out]
        ctx.actions.run_shell(
            outputs = [out],
            inputs = depset([f]),
            arguments = [f.path, out.path],
            command = "cp $1 $2",
        )

    # Small sanity check
    if len(all_input_files) != len(all_outputs):
        fail("Output count should be 1-to-1 with input count.")

    return [
        DefaultInfo(
            files = depset(all_outputs),
            runfiles = ctx.runfiles(files = all_outputs),
        ),
    ]

copy_files_new_dir = rule(
    implementation = _copy_files_new_dir_impl,
    attrs = {
        "source_files": attr.label_list(),
        "base_dir": attr.string(),
        "prefix": attr.string(),
        "no_prefix": attr.bool(
            default = False,
        ),
    },
)

_TESTS = [
    "*/src/**/*.test.js*",
    "*/src/**/*.test.ts*",
    "*/src/**/*.spec.js*",
    "*/src/**/*.spec.ts*",
    "*/src/**/__tests__/**/*.js*",
    "*/src/**/__tests__/**/*.ts*",
]

def _resolve_files(ctx):
    print(ctx.attrs.files.data.to_list())
    return []

resolve_files = rule(
    _resolve_files,
    attrs = {
        "files": attr.label(),
    },
)

def react_project(name, srcs, public, package_json, yarn_lock):
    runner_dir_name = name + "-runner"
    merge_json_name = name + "-merge-json"
    native.genrule(
        name = merge_json_name,
        outs = [paths.join(runner_dir_name, "package.json")],
        tools = ["//bazel/ui:merge-package.sh"],
        cmd = "$(location //bazel/ui:merge-package.sh) $(SRCS) > $@",
        srcs = [
            package_json,
            "//ui:package.json",
        ],
    )
    copy_srcs_name = name + "-copy-srcs"
    copy_files_new_dir(
        name = copy_srcs_name,
        source_files = [
            srcs,
        ],
        prefix = "src",
        base_dir = runner_dir_name,
    )
    copy_public_name = name + "-copy-public"
    copy_files_new_dir(
        name = copy_public_name,
        source_files = [
            public,
        ],
        prefix = "public",
        base_dir = runner_dir_name,
    )

    copy_extras_name = name + "-copy-extras"
    copy_files_new_dir(
        name = copy_extras_name,
        source_files = [
            "//ui:ui-extras",
        ],
        no_prefix = True,
        base_dir = runner_dir_name,
    )

    chdir_script_name = name + "-write-chdir-script"
    write_file(
        name = chdir_script_name,
        out = paths.join(runner_dir_name, "chdir.js"),
        content = ["process.chdir(__dirname)"],
    )

    _RUNTIME_DEPS = [
        "@npm//react",
        "@npm//react-dom",
        copy_public_name,
        copy_srcs_name,
        merge_json_name,
        chdir_script_name,
        copy_extras_name,
    ]
    react_scripts(
        name = name + "-start",
        args = [
            "--node_options=--require=./$(rootpath :" + chdir_script_name + ")",
            "start",
        ],
        data = _RUNTIME_DEPS,
        tags = [
            # This tag instructs ibazel to pipe into stdin a event describing actions.
            # ibazel send EOF to stdin by default and `react-scripts start` will stop when getting EOF in stdin.
            # So use this to prevent EOF.
            "ibazel_notify_changes",
        ],
    )

    react_scripts(
        name = name + "-build",
        args = [
            "--node_options=--require=./$(rootpath :" + chdir_script_name + ")",
            "build",
        ],
        data = _RUNTIME_DEPS,
        tags = [
            # This tag instructs ibazel to pipe into stdin a event describing actions.
            # ibazel send EOF to stdin by default and `react-scripts start` will stop when getting EOF in stdin.
            # So use this to prevent EOF.
            "ibazel_notify_changes",
        ],
    )
