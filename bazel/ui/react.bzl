load("@npm//react-scripts:index.bzl", "react_scripts", "react_scripts_test")
load("@build_bazel_rules_nodejs//:index.bzl", "copy_to_bin", "nodejs_test")
load("@bazel_skylib//rules:write_file.bzl", "write_file")
load("@build_bazel_rules_nodejs//:index.bzl", "nodejs_binary")
load("//bazel/utils:files.bzl", "rebase_and_copy_files")
load("@bazel_skylib//lib:paths.bzl", "paths")
load("//bazel/utils:binary.bzl", "declare_binary")

"""Creates a react project after running create-react-app.

Args:
    srcs: a list of targets, usual tsx and ts files
    package_jsons: a list of package.jsons to be zipped. Does nto check for conflics
    yarn_locks: matching list of yarn.locks for the package.jsons
    publics: public/ folder of CRA. Can be zipped together
    tsconfig: singular tsconfig.json
    patches: list of git-patchs wanted to apply.

Main Commands:
    >>> ibazel run //<dir>:<name>-start => starts webpack server with hot reloading
    >>> bazel build //<dir>:<name>-build => builds for production
    >>> bazel test //<dir>:<name>-test => runs tests

"""

# TODO(adam): fail macro on conflict of yarn lock. cannot have same deps in different package jsons
def react_project(name, srcs, package_jsons, yarn_locks, publics, tsconfig, patches, **kwargs):
    runner_dir_name = name + "-runner"
    merge_json_name = name + "-merge-json"
    native.genrule(
        name = merge_json_name,
        outs = [paths.join(runner_dir_name, "package.json")],
        tools = ["//bazel/ui:merge-package.sh", "//bazel/ui:jq"],
        cmd = "$(location //bazel/ui:merge-package.sh) $(location //bazel/ui:jq) $(SRCS) > $@",
        srcs = package_jsons,
        **kwargs
    )
    copy_srcs_name = name + "-copy-srcs"
    rebase_and_copy_files(
        name = copy_srcs_name,
        source_files = srcs,
        prefix = "src",
        base_dir = runner_dir_name,
        **kwargs
    )
    copy_public_name = name + "-copy-public"
    rebase_and_copy_files(
        name = copy_public_name,
        source_files = publics,
        prefix = "public",
        base_dir = runner_dir_name,
        **kwargs
    )
    native.filegroup(
        name = name + "-ui-extras",
        srcs = yarn_locks + [
            tsconfig,
        ],
        **kwargs
    )

    copy_extras_name = name + "-copy-extras"
    rebase_and_copy_files(
        name = copy_extras_name,
        source_files = [
            name + "-ui-extras",
        ],
        base_dir = runner_dir_name,
    )
    rebase_and_copy_files(
        name = name + "-copy-patches",
        source_files = patches,
        prefix = "patches",
        base_dir = runner_dir_name,
        **kwargs
    )

    chdir_script_name = name + "-write-chdir-script"
    write_file(
        name = chdir_script_name,
        out = paths.join(runner_dir_name, "chdir.js"),
        content = ["process.chdir(__dirname)"],
        **kwargs
    )

    _RUNTIME_DEPS = [
        copy_public_name,
        copy_srcs_name,
        merge_json_name,
        chdir_script_name,
        copy_extras_name,
        name + "-copy-patches",
        "@npm//:node_modules",
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
        **kwargs
    )

    react_scripts(
        # Note: If you want to change the name make sure you update BUILD_PATH below accordingly
        # https://create-react-app.dev/docs/advanced-configuration/
        name = name + "-build",
        args = [
            "--node_options=--require=./$(execpath :" + chdir_script_name + ")",
            "build",
        ],
        data = _RUNTIME_DEPS + [
            "@npm//@types",
        ],
        env = {
            "BUILD_PATH": "./build",
        },
        output_dir = True,
        **kwargs
    )

    react_scripts_test(
        name = name + "-test",
        args = [
            "--node_options=--require=./$(rootpath :" + chdir_script_name + ")",
            "test",
            "--watchAll=false",
            "--no-cache",
            "--no-watchman",
            "--ci",
            "--debug",
        ],
        data = _RUNTIME_DEPS,
        # Need to set the pwd to avoid jest needing a runfiles helper
        # Windows users with permissions can use --enable_runfiles
        # to make this test work
        tags = ["no-bazelci-windows"],
        **kwargs
    )
