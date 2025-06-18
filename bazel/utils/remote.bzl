"""
Simple rules to export targets and their inputs to a local or remote
directory, and to allow running targets on remote machines.

Defaults to using rsync + ssh.

Basic usage is trivial. For example, by defining:

    sh_test( 
        name = "my_test",
        ...
    )

    remote_run(
        name = "test-on-remote-machine",
        target = ":my_test",
        dests = ["test-machine-00.corp"],
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

    ./wrapper --flag_to_my_wrapper -- ./whatever/path/to/my_test --flag_to_my_test

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

The remote_run rule also supports an `inputs =` attribute, usable in alternative
to the `target` attribute described above.

If the inputs attribute is used, instead of copying or executing the files generated
by a rule, it copies the **input** files required for the rule to run.

For example, let's say you have a:

    cc_library(
        name = "my-library",
        srcs = ["tester.cc", "processor.cc"],
        hdrs = ["my-library.h"],
    )

A rule like:

    export(
        name = "export-my-library",
        inputs = ":my-library,
    )

would copy out 'tester.cc', 'processor.cc', 'my-library.h' instead of the generated
.so and .a files that `target = ":my-library"` would have exported.
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

SuiteTests = provider(
    doc = "Represents the tests in a test suite - generally provider by the suite_aspect",
    fields = {
        "tests": "List of tests in the suite",
    },
)

def _suite_aspect(target, ctx):
    if ctx.rule.kind == "test_suite":
        return [SuiteTests(tests = ctx.rule.attr.tests)]
    return []

suite_aspect = aspect(
    doc = """Detects whether a target is a test suite.""",
    implementation = _suite_aspect,
    attr_aspects = [],
)

InputFiles = provider(
    doc = "Represents the input files of a rule - generally provider by the inputs_aspect",
    fields = {
        "direct": "List of all the direct input files of a rule",
        "all": "List of all the input files of a rule and its dependencies",
    }
)

def _inputs_aspect(target, ctx):
    # Why hardcode the noop string below? When the other targets allow an arbitrary noop?
    #
    # Aspects can only take a string attribute, from a list of possible values. There's no
    # reasonable way to make it configurable.
    if "//bazel/utils/remote:noop" in package(target.label):
        return []

    direct = []
    allfiles = []
    files = ctx.rule.files
    for attr in dir(files):
        locfiles = getattr(files, attr, [])
        # ctx.rule.files by  contract is a struct, with each attribute
        # being a list, except for to_json and to_proto. Skip the two methods.
        # Anything else that's not iterable will cause a crash.
        if type(locfiles) == "builtin_function_or_method":
            continue

        direct.extend(locfiles)
        allfiles.extend(locfiles)

        attrvalue = getattr(ctx.rule.attr, attr)
        if type(attrvalue) != "list":
            attrvalue = [attrvalue]

        for atarget in attrvalue:
            if atarget and InputFiles in atarget:
                allfiles.extend(atarget[InputFiles].all.to_list())

    return [InputFiles(direct = depset(direct), all = depset(allfiles))]

inputs_aspect = aspect(
    doc = """Adds an InputFiles provider with all the inputs needed by a target.

If the target is @enkit//bazel/utils/remote:noop, no InputFiles provider is returned.
This is useful to allow the parameter to be configured via a label_flag()""",
    implementation = _inputs_aspect,
    attr_aspects = ["*"],
)

_known_attributes = [
    ("wrapper_opts", attr.string_list, dict(
        default = [],
        doc = "Command line options to supply to the wrapper, if any",
    )),
    ("target_opts", attr.string_list, dict(
        default = [],
        doc = "Additional command line options to pass to the target on the remote machine",
    )),
    ("noop", attr.label, dict(
        executable = True,
        cfg = "host",
        default = "@enkit//bazel/utils/remote:noop",
        doc = "If this target is used as wrapper or as a target, it is consider a noop operation. Useful when target or wrapper as specified via label_flag()",
    )),
    ("rsync_cmd", attr.string, dict(
        default = "rsync",
        doc = "Command to run in order to execute rsync - can refer to system binaries (non-hermetic) or binaries brought in via the 'tools' attribute",
    )),
    ("rsync_opts", attr.string_list, dict(default = [
        "--delete",
        "-avrz",
        "--progress",
        "--copy-unsafe-links",
    ], doc = "Flags to pass to the rsync binary")),
    ("destdir", attr.string, dict(
        default = "~",
        doc = "Destination directory where to copy data into",
    )),
    ("ssh_cmd", attr.string, dict(
        default = "ssh",
        doc = "Command to run in order to execute ssh - can refer to system binaries (non-hermetic) or binaries brought in via the 'tools' attribute",
    )),
    ("ssh_opts", attr.string_list, dict(default = [
    ], doc = "Additional flags to pass to the ssh binary")),
    ("dests", attr.string_list, dict(default = [
    ], doc = """\
List of destinations to copy the output to.

When the target is executable, the target is run from the first destination supplied.
If no destination is supplied, at least one destination MUST be supplied manually
when invoking the target (use `bazel run //name/of:target -- -h` to see the options available).

The destination can be any string accepsted by the rsync_cmd. If it's a string
containing no ':' and no '/', it is assumed to be a machine name, and data is copied
to `machine:{destdir}/{targetname with invalid characters replaced with _}`, where
`destdir` is a parameter to this rule, and defaults to the user home directory.

Any other string is interpreted as a literal path. If interpreted as literal path,
no ssh command is invoked - if the target is executable, it is run directly.""")),
    ("only_copy", attr.bool, dict(default = False, doc = "If true, does not execute any target on the remote machine.")),
    ("template", attr.label, dict(
        default = "@enkit//bazel/utils/remote:runner.template.sh",
        allow_single_file = True,
        doc = "template to use to generate the shell script to run the target remotely",
    )),
    ("tools", attr.label_list, dict(
        allow_files = True,
        cfg = "host",
        doc = "Additional tools to require on YOUR host to perform the copy. " +
              "This is useful - for example - to use a custom rsync or ssh binary (specify a bazel relative path as ssh_bin, referencing the binary generated by the included target)",
    )),
    ("alldeps", attr.bool, dict(default = False, doc = """\
If true, includes ALL deps.

For targets specified with `target =`, this means including both runfiles
and DefaultInfo(files =) (hint: this solves the sh_binary data dependency
bug, https://github.com/bazelbuild/bazel/issues/12348).

For targets specified via `inputs =`, this means that all recurisve inputs
are brought in, not just the direct inputs for the target specified.
""")),
]

def _common_attrs(default = True):
    """Generates the list of attributes common to a few rules.

    Why is this function needed? We have two rules that need the same
    attribute. But one rule needs to know which parameters were actually
    set by the user, and if an attribute has a default, there's no
    possible way in code to tell apart a default from a value that a
    user actually set.

    This function can generate the list of common attributes with or
    without a defined default.

    Args:
      default: boolean, if set to True, include the default flag value.
    """
    result = {}
    for name, constructor, kwargs in _known_attributes:
        if not default:
            kwargs = dict(**kwargs)
            kwargs.pop("default", None)

        result[name] = constructor(**kwargs)
    return result

def _common_attrs_to_dict(ctx, default = True):
    """Converts all the common attributes into a dictionary.

    Args:
      default: boolean, if set to False, attributes with a default empty
               value (python false definition) are not added to the dict.
    """
    attrs = {}
    for name, _, _ in _known_attributes:
        # starlark objects do not support the 'in' operator, only dicts.
        value = getattr(ctx.attr, name, None)
        if value == None or (not default and not value):
            continue
        attrs[name] = value

    return attrs

def _export_and_run_impl(ctx):
    has_wrapper = ctx.attr.wrapper and not package(ctx.attr.wrapper.label) == package(ctx.attr.noop.label)

    attrs = _common_attrs_to_dict(ctx)
    if has_wrapper and RemoteWrapper in ctx.attr.wrapper:
        attrs = merge_kwargs(attrs, ctx.attr.wrapper[RemoteWrapper].attributes)
    attrs = struct(**attrs)

    input_files = []
    target_label = ""
    if ctx.attr.inputs and InputFiles in ctx.attr.inputs:
        if attrs.alldeps:
            input_files = ctx.attr.inputs[InputFiles].all.to_list()
        else:
            input_files = ctx.attr.inputs[InputFiles].direct.to_list()
        target_label = ctx.attr.inputs.label

    if not input_files and (not ctx.attr.target or package(ctx.attr.target.label) == package(attrs.noop.label)):
        fail(location(ctx) + "A target must be supplied via flags - check '//" + ctx.build_file_path + "' for details")

    tdi = None
    env = {}
    if ctx.attr.target and package(ctx.attr.target.label) != package(attrs.noop.label):
        if input_files:
            fail(location(ctx) + "Either a target or inputs must be supplied via flags - not both! Check '//" + ctx.build_file_path + "' for details")

        tdi = ctx.attr.target[DefaultInfo]
        target_label = ctx.attr.target.label
        if SuiteTests in ctx.attr.target:
            fail("{} is a test suite.  It expands to the following tests: {}.  Use one of these targets instead.".format(target_label, ", ".join([str(t) for t in ctx.attr.target[SuiteTests].tests])))
        input_files = tdi.files.to_list()
        if RunEnvironmentInfo in ctx.attr.target:
            env = ctx.attr.target[RunEnvironmentInfo].environment

    runfiles = ctx.runfiles()
    target_opts = attrs.target_opts
    target_exec = ""
    if hasattr(tdi, "files_to_run") and hasattr(tdi.files_to_run, "executable") and tdi.files_to_run.executable:
      no_execute = False
      target_exec = tdi.files_to_run.executable.short_path
      target_runfiles = tdi.default_runfiles

      runfiles = runfiles.merge(ctx.runfiles(files = [tdi.files_to_run.executable]))
      runfiles = runfiles.merge(target_runfiles)
      if attrs.alldeps:
          runfiles = runfiles.merge(ctx.runfiles(files = input_files))

      if has_wrapper:
          wrapper_exec = ctx.attr.wrapper[DefaultInfo].files_to_run.executable
          wrapper_runfiles = ctx.attr.wrapper[DefaultInfo].default_runfiles
          runfiles = runfiles.merge(wrapper_runfiles)
          runfiles = runfiles.merge(ctx.runfiles(files = [wrapper_exec]))

          target_opts = attrs.wrapper_opts + ["--", target_exec] + target_opts
          target_exec = wrapper_exec.short_path
    else:
      no_execute = True
      runfiles = runfiles.merge(ctx.runfiles(files = input_files))

    include = ctx.outputs.include
    ctx.actions.write(include, "\n".join([ctx.workspace_name + "/" + f.short_path for f in runfiles.files.to_list()]))

    subs = dict(
        include = shell.quote(include.short_path),
        destdir = shell.quote(attrs.destdir),
        target = shell.quote(package(target_label)),
        target_opts = shell.array_literal(target_opts),
        executable = shell.quote(target_exec),
        env = " ".join(["{}={}".format(e, env[e]) for e in env]),
        no_execute = (no_execute and "true") or "",
        workspace = shell.quote(ctx.workspace_name),
        rsync_opts = shell.array_literal(attrs.rsync_opts),
        rsync_cmd = shell.quote(attrs.rsync_cmd),
        ssh_opts = shell.array_literal(attrs.ssh_opts),
        ssh_cmd = shell.quote(attrs.ssh_cmd),
        dests = shell.array_literal(attrs.dests),
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
        if di.default_runfiles:
            runfiles = runfiles.merge(di.default_runfiles)

    return DefaultInfo(files = depset([runner, include]), executable = runner, runfiles = runfiles)

export_and_run_rule = rule(
    implementation = _export_and_run_impl,
    executable = True,
    attrs = dict(_common_attrs(), **{
        "target": attr.label(
            cfg = "target",
            doc = "Target to execute on the remote machine",
            aspects = [suite_aspect],
        ),
        "inputs": attr.label(
            doc = "Target whose input files need copied or exported - only one of target = or inputs = is allowed",
            aspects = [inputs_aspect],
        ),
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
    # Executable rules in bazel are *required* to generate a new binary :(. Otherwise, we see the following error:
    #   <builtin>: 'executable' provided by an executable rule 'remote_wrapper' should be created by the same rule.
    #
    # Would have loved to just return ctx.attr.wrapper[DefaultInfo] instead.
    di = ctx.attr.wrapper[DefaultInfo]
    executable = di.files_to_run.executable
    runfiles = di.default_runfiles

    ctx.actions.symlink(output = ctx.outputs.script, target_file = executable, is_executable = True)
    return [
        DefaultInfo(
            executable = ctx.outputs.script,
            files = depset(direct = [ctx.outputs.script], transitive = [runfiles.files]),
            runfiles = runfiles,
        ),
        RemoteWrapper(attributes = _common_attrs_to_dict(ctx, default = False)),
    ]

remote_wrapper_rule = rule(
    implementation = _remote_wrapper,
    executable = True,
    attrs = dict(_common_attrs(default = False), **{
        "wrapper": attr.label(
            executable = True,
            cfg = "host",
            mandatory = True,
            doc = "A target generating a binary to be run on the REMOTE system with the real target and its arguments as argv[1] and on. " +
                  "This is useful to - for example - create a tool to setup a chroot or container or use docker to run the command remotely",
        ),
        "script": attr.output(mandatory = True, doc = "New name of the generated binary"),
    }),
)

def remote_wrapper(name, **kwargs):
    remote_wrapper_rule(name = name, script = name + "-wrapper", **kwargs)

def remote_run(name, **kwargs):
    """Defines a target to run a specific target on a remote machine."""
    export_and_run_rule(name = name, script = name + "-copy-and-run.sh", include = name + ".files_to_copy", **kwargs)

def export(name, **kwargs):
    """Defines a target to export files by a target in a specified directory."""
    export_and_run_rule(name = name, script = name + "-export.sh", include = name + ".files_to_copy", only_copy = True, **kwargs)
