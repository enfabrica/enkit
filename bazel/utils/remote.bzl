"""
Simple rules to allow running targets on remote machines using rsync + ssh.

Basic usage is trivial. For example, by defining:

    sh_test( 
        name = "my_test",
        ...
    )

    remote_run(
        name = "test-on-remote-machine",
        target = ":my_test",
        machines = ["test-machine-00.corp"],
    )

You can then run `bazel run :test-on-remote-machine` which will result in:
  1) Target "my_test" being built, including all its dependencies.
  2) The target, and all its dependencies, copied on the remote machine using
     rysnc, in a directory tree that matches the bazel structure.
  3) The target being run on the remote machine.

The rule relies on 'rsync' and 'ssh' being available on both your machine
and the remote system (not hermetic - but can be made hermetic using the
'tools' attribute).

Partial parametrization is allowed. For example, you can have a rule like:

    remote_run(
        name = "test-on-remote-machine",
        target = ":my_test",
    )


And use `bazel run :test-on-remote-machine -- test-machine-00.corp` to achieve
the same effect as per first rule.

You can use `bazel run :test-on-remote-machine -- -h` to see all the options
supported by the generated `copy and run` magic shell script.

Combined with label_flag(), `remote_run` allows to create a single generic
remote target for any target in your repository. For example, in your top
level BUILD.bazel file you can define:
    
    label_flag(
        name = "remote_target",
        build_setting_default = "@enkit//bazel/utils/remote:noop",
    )

    remote_run(
        name = "run-remotely",
        target = ":remote_target,
    )

which would allow you to run:

    bazel run //:run-remotely --//:remote_target=//whatever/path/to:my_test -- test-machine-00.corp

with any arbitrary target. If you use the `:noop` target as shown above for flag
defaults, the `remote_run` rule will be able to detect the lack of a specified
`--//:remote_target` parameter and print a helpful message and stop, rather than
continue with an invalid / unspecified target.

The remote_run() rule also supports specifying a `wrapper`. A `wrapper` is copied
and run ON THE REMOTE machine just like the target itself. The `wrapper` is passed
the original command line of the target, including the options, eg, think of the
wrapper as something invoked on the remote machine as:

    ./wrapper ./whatever/path/to/my_test -- --flag_to_my_test

With some custom wrapper flags (specified in wrapper_opts). The wrapper can also
return a RemoteWrapper() provider to override the flags of the rule itself, so
for example, to override the remote installation directory.

Expanding on the previous example:

    label_flag(
        name = "remote_target",
        build_setting_default = "@enkit//bazel/utils/remote:noop",
    )

    sh_binary(
        name = "run_in_docker_helper",
        srcs = [ ... ] # A shell script running commands using docker exec.
    )

    remote_wrapper(
        name = "run_in_docker",
        destdir = "~/docker-home/$USER",
    )

    sh_binary(
        name = "run_baremetal",
        srcs = [ ... ] # A shell script just running the commands.
    )

    label_flag(
        name = "remote_wrapper",
        build_setting_default = ":run_in_docker",
    )

    remote_run(
        name = "run-remotely",
        target = ":remote_target,
    )

Would allow to run:

    bazel run //:run-remotely --//:remote_target=//whatever/path/to:my_test \
      //:remote_wrapper=//:run_in_docker -- test-machine-00.corp

or:

    bazel run //:run-remotely --//:remote_target=//whatever/path/to:my_test \
      //:remote_wrapper=//:run_baremetal -- test-machine-00.corp

to allow running either baremetal or in docker.
"""

load("@bazel_skylib//lib:shell.bzl", "shell")
load("//bazel/utils:messaging.bzl", "location", "package")
load("//bazel/utils:merge_kwargs.bzl", "merge_kwargs")

RemoteWrapper = provider(
    doc = "Optional provider a wrapper can return to supply parameters for remote_run",
    fields = {
        "attributes": """
dict of string keys, where the names match attributes of the remote_run rule.

Allows to carry parameters like ssh options, or a specific destination
directory that a wrapper requires by default to be used.
""",
    },
)

_common_attrs = {
    "wrapper_opts": attr.string_list(
        default = [],
        doc = "Command line options to supply to the wrapper, if any",
    ),
    "target": attr.label(
        executable = True,
        cfg = "host",
        mandatory = True,
        doc = "Target to execute on the remote machine",
    ),
    "target_opts": attr.string_list(
        default = [],
        doc = "Additional command line options to pass to the target on the remote machine",
    ),
    "noop": attr.label(
        executable = True,
        cfg = "host",
        default = "@enkit//bazel/utils/remote:noop",
        doc = "If this target is used as wrapper or as a target, it is consider a noop operation. Useful when target or wrapper as specified via label_flag()",
    ),
    "rsync_bin": attr.string(
        default = "rsync",
        doc = "Path to a binary to run as rsync - assumed to be installed on the system",
    ),
    "rsync_opts": attr.string_list(default = [
        "--delete",
        "-avrz",
        "--progress",
        "--copy-unsafe-links",
    ], doc = "Flags to pass to the rsync binary"),
    "destdir": attr.string(
        default = "~",
        doc = "Destination directory where to copy data into",
    ),
    "ssh_bin": attr.string(
        default = "ssh",
        doc = "Path to a binary to run as ssh - assumed to be installed on the system",
    ),
    "ssh_opts": attr.string_list(default = [
    ], doc = "Additional flags to pass to the ssh binary"),
    "machines": attr.string_list(default = [
    ], doc = "List of machines to copy the output to, target is run on the first machine listed. If not supplied, it must be supplied at run time when invoking the target."),
    "only_copy": attr.bool(default = False, doc = "If true, does not execute any target on the remote machine."),
    "template": attr.label(
        default = "@enkit//bazel/utils/remote:runner.template.sh",
        allow_single_file = True,
        doc = "template to use to generate the shell script to run the target remotely",
    ),
    "tools": attr.label_list(
        allow_files = True,
        cfg = "host",
        doc = "Additional tools to require on YOUR host to perform the copy. " +
              "This is useful - for example - to use a custom rsync or ssh binary (specify a bazel relative path as ssh_bin, referencing the binary generated by the included target)",
    ),
}

def _common_attrs_to_dict(ctx):
    """Converts all the common attributes into a dictionary."""
    attrs = {}
    for name in _common_attrs:
        value = getattr(ctx.attr, name, None)
        if value == None:
            continue
        attrs[name] = value

    return attrs

def _remote_run_impl(ctx):
    has_wrapper = ctx.attr.wrapper and not package(ctx.attr.wrapper.label) == package(ctx.attr.noop.label)

    attrs = _common_attrs_to_dict(ctx)
    if has_wrapper and RemoteWrapper in ctx.attr.wrapper:
        attrs = merge_kwargs(attrs, ctx.attr.wrapper[RemoteWrapper].attributes)

    attrs = struct(**attrs)
    if package(attrs.target.label) == package(attrs.noop.label):
        fail(location(ctx) + "A target must be supplied via flags - read the file '//" + ctx.build_file_path + "' for details")

    target_exec = attrs.target[DefaultInfo].files_to_run.executable
    target_runfiles = attrs.target[DefaultInfo].default_runfiles
    target_opts = attrs.target_opts

    runfiles = ctx.runfiles(files = [target_exec])
    runfiles = runfiles.merge(target_runfiles)

    if has_wrapper:
        wrapper_exec = ctx.attr.wrapper[DefaultInfo].files_to_run.executable
        wrapper_runfiles = ctx.attr.wrapper[DefaultInfo].default_runfiles
        runfiles = runfiles.merge(wrapper_runfiles)
        runfiles = runfiles.merge(ctx.runfiles(files = [wrapper_exec]))

        target_opts = attrs.wrapper_opts + ["--", target_exec] + target_opts
        target_exec = wrapper_exec

    include = ctx.outputs.include
    ctx.actions.write(include, "\n".join([ctx.workspace_name + "/" + f.short_path for f in runfiles.files.to_list()]))

    subs = dict(
        include = shell.quote(include.short_path),
        destdir = shell.quote(attrs.destdir),
        target = shell.quote(package(attrs.target.label)),
        target_opts = shell.array_literal(attrs.target_opts),
        executable = shell.quote(target_exec.short_path),
        workspace = shell.quote(ctx.workspace_name),
        rsync_opts = shell.array_literal(attrs.rsync_opts),
        rsync_bin = shell.quote(attrs.rsync_bin),
        ssh_opts = shell.array_literal(attrs.ssh_opts),
        ssh_bin = shell.quote(attrs.ssh_bin),
        machines = shell.array_literal(attrs.machines),
        only_copy = (attrs.only_copy and "true") or "",
    )

    runner = ctx.outputs.script
    template = attrs.template
    ctx.actions.expand_template(
        template = template[DefaultInfo].files.to_list()[0],
        output = runner,
        substitutions = dict([("{" + k + "}", v) for k, v in subs.items()]),
        is_executable = True,
    )

    runfiles = runfiles.merge(ctx.runfiles(files = [include]))
    for tool in attrs.tools:
        di = tool[DefaultInfo]
        runfiles = runfiles.merge(ctx.runfiles(files = di.files.to_list()))
        if di.runfiles:
            runfiles = runfiles.merge(di.runfiles)

    return DefaultInfo(files = depset([runner, include]), executable = runner, runfiles = runfiles)

remote_run_rule = rule(
    implementation = _remote_run_impl,
    executable = True,
    attrs = dict(_common_attrs, **{
        "wrapper": attr.label(
            executable = True,
            cfg = "host",
            doc = "A target generating a binary to be run on the REMOTE system with the real target and its arguments as argv[1] and on. " +
                  "This is useful to - for example - create a tool to setup a chroot or container or use docker to run the command remotely",
        ),
        "include": attr.output(mandatory = True, doc = "Name of the generated list of files to copy to the remote machine"),
        "script": attr.output(mandatory = True, doc = "Name of the generated script to run to perform the copy and exec the target"),
    }),
)

def _remote_wrapper(ctx):
    return [ctx.attr.wrapper[DefaultInfo], RemoteWrapper(attributes = _common_attrs_to_dict(ctx))]

remote_wrapper = rule(
    implementation = _remote_wrapper,
    executable = True,
    attrs = dict(_common_attrs, **{
        "wrapper": attr.label(
            executable = True,
            cfg = "host",
            mandatory = True,
            doc = "A target generating a binary to be run on the REMOTE system with the real target and its arguments as argv[1] and on. " +
                  "This is useful to - for example - create a tool to setup a chroot or container or use docker to run the command remotely",
        ),
    }),
)

def remote_run(name, **kwargs):
    remote_run_rule(name = name, script = name + "-copy-and-run.sh", include = name + ".files_to_copy")
