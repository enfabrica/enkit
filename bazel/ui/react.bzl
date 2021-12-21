load("@npm//react-scripts:index.bzl", "react_scripts", "react_scripts_test")
load("@build_bazel_rules_nodejs//:index.bzl", "copy_to_bin", "nodejs_test")
load("@bazel_skylib//rules:write_file.bzl", "write_file")
load("@build_bazel_rules_nodejs//:index.bzl", "nodejs_binary")
load("//bazel/utils:files.bzl", "rebase_and_copy_files")
load("@bazel_skylib//lib:paths.bzl", "paths")
load("//bazel/utils:binary.bzl", "declare_binary")
load("@npm//poi:index.bzl", "poi")
load("@npm//webpack:index.bzl", "webpack")
load("@npm//jest:index.bzl", "jest")
"""Creates a react project after running create-react-app.

Args:
    src: a label of the filegroup under src/, usually tsx and ts files
    package_jsons: a list of package.jsons to be zipped. Does not check for conflicts (yet)
    copy_to_root: list of files (usually config files) that are blindly copied to the root of the projects build.
    public: public/ folder used for the webpack server. Can be zipped together
    tsconfig: singular tsconfig.json
    patches: list of git-patchs wanted to apply.
    includes: a list of dics, ad hco files to copy {to: "<workspace exclusive>"/"path prefix" from: "existing", labels: ["list of labels] }

Main Commands:
    >>> bazel run //<dir>:<name>-start => starts webpack server with hot reloading enabled
    >>> bazel build //<dir>:<name>-build => builds for production
    >>> bazel test //<dir>:<name>-test => runs tests

"""

# TODO(adam): fail macro on conflict of yarn lock. cannot have same deps in different package jsons
def react_project(name, src, package_jsons, copy_to_root, public, tsconfig, patches, includes, debug = False, **kwargs):
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
        source_files = [src],
        prefix = "src",
        base_dir = runner_dir_name,
        **kwargs
    )

    copy_public_name = name + "-copy-public"
    rebase_and_copy_files(
        name = copy_public_name,
        source_files = public,
        prefix = "public",
        base_dir = runner_dir_name,
        **kwargs
    )
    copy_root_filegroup_name = name + "-copy-roots-filegroup"
    native.filegroup(
        name = copy_root_filegroup_name,
        srcs = copy_to_root + [tsconfig],
    )
    copy_roots_name = name + "-copy-roots"
    rebase_and_copy_files(
        name = copy_roots_name,
        source_files = [copy_root_filegroup_name],
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

    index = 0
    copy_includes_name = "copy-includes-"
    includes_targets = []
    for i in includes:
        dir_name = paths.join(runner_dir_name, i["to"])
        includes_targets.append(copy_includes_name + str(index))
        rebase_and_copy_files(
            name = copy_includes_name + str(index),
            source_files = i["labels"],
            prefix = i["from"],
            base_dir = dir_name,
            **kwargs
        )

    _RUNTIME_DEPS = [
        copy_public_name,
        copy_srcs_name,
        merge_json_name,
        chdir_script_name,
        copy_roots_name,
        name + "-copy-patches",
        "@npm//:node_modules",
    ] + includes_targets

    run_args = ["--node_options=--require=./$(rootpath :" + chdir_script_name + ")", "--serve"]
    if debug:
        run_args += ["--inspect-webpack"]

    webpack(
        name = name + "-start",
        args = ["serve", "--config=./webpack.dev.js"],
        data = _RUNTIME_DEPS,
        tags = [
            # This tag instructs ibazel to pipe into stdin a event describing actions.
            # ibazel send EOF to stdin by default and `react-scripts start` will stop when getting EOF in stdin.
            # So use this to prevent EOF.
            "ibazel_notify_changes",
        ],
        chdir = paths.join(native.package_name(), runner_dir_name),
        **kwargs
    )
    build_target_name = name + "-build"
    webpack(
        name = name + "-build",
        args = [
            "--node_options=--max_old_space_size=4096",
            "--config=./webpack.prod.js",
        ],
        data = _RUNTIME_DEPS + [
            "@npm//@types",
        ],
        output_dir = True,
        env = {
            "BUILD_DIR": paths.join("../" + build_target_name),
        },
        chdir = paths.join("bazel-out/k8-fastbuild/bin", native.package_name(), runner_dir_name),
        **kwargs
    )

    jest(
        name = name + "-test",
        args = [
            "--test",
            "--no-cache",
        ],
        data = _RUNTIME_DEPS,
        # Need to set the pwd to avoid jest needing a runfiles helper
        # Windows users with permissions can use --enable_runfiles
        # to make this test work
        tags = ["no-bazelci-windows"],
        chdir = paths.join(native.package_name(), runner_dir_name),
        **kwargs
    )
