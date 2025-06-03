load("@bazel_skylib//lib:shell.bzl", "shell")
load("//bazel/linux:providers.bzl", "KernelBundleInfo", "KernelImageInfo", "RuntimeBundleInfo", "RuntimeInfo")
load("//bazel/linux:runner.bzl", "get_prepare_run_check", "runtime_info_from_target")
load("//bazel/linux:utils.bzl", "expand_deps", "get_compatible")
load("//bazel/utils:files.bzl", "files_to_dir")
load("//bazel/utils:messaging.bzl", "location", "package")

def _add_attr_bundle(ctx, bundle, name, merge = [], distribute = []):
    """Used by _vm_bundle, helper to parase its attributes"""
    abin = getattr(ctx.attr, name + "_bin")
    abins = getattr(ctx.attr, name, [])
    aargs = getattr(ctx.attr, name + "_args", [])
    acmds = getattr(ctx.attr, name + "_cmds", [])

    if not abin and not acmds and not abins:
        return

    if abins and abin:
        fail(location(ctx) + "defines both {name}_bin and {name} - only one is allowed".format(name = name))
    if abins and aargs:
        fail(location(ctx) + "defines both {name} and {name}_args - {name}_args is only allowed with {name}_bin".format(name = name))
    if aargs and not abin:
        fail(location(ctx) + "defines {name}_args but not {name}_bin - {name}_args is only allowed with {name}_bin".format(name = name))
    if aargs and RuntimeBundleInfo in abin:
        fail(location(ctx) + "has {name}_bin pointing to a vm_bundle and also defines {name}_args - which is not allowed".format(name = name))

    rtis = []

    # If abin has no arguments, it is allowed to be a bundle. Rely on expand_targets_and_bundles
    # to compute the actual RuntimeInfo to use.
    if abin and not aargs:
        abins = [abin] + abins
        abin = None
    if abin or aargs or acmds:
        rtis += [runtime_info_from_target(ctx, abin, commands = acmds, args = aargs)]

    if abins:
        # Why action=name? Naked binaries (not bundles!) listed in the specific attribute
        # need to be assigned to that specific step. Eg, if we're processing the "prepare"
        # actions, a binary outside a bundle should be considered a "prepare" command, not
        # a run command. And those binaries are allowed to appear anywhere.
        bundles = get_prepare_run_check(ctx, abins, action = name)
        rtis.extend(getattr(bundles, name, []))
        for step in merge:
            rtis.extend(getattr(bundles, step, []))
        for step in distribute:
            bundle.setdefault(step, []).extend(getattr(bundles, step))

    bundle.setdefault(name, []).extend(rtis)

def _vm_bundle(ctx):
    bundle = {}
    _add_attr_bundle(ctx, bundle, "prepare", distribute = ["cleanup", "check"])
    _add_attr_bundle(ctx, bundle, "init", merge = ["run"], distribute = ["prepare", "cleanup", "check"])
    _add_attr_bundle(ctx, bundle, "run", distribute = ["init", "prepare", "cleanup", "check"])
    _add_attr_bundle(ctx, bundle, "cleanup", distribute = ["prepare", "check"])
    _add_attr_bundle(ctx, bundle, "check", distribute = ["prepare", "cleanup"])

    return RuntimeBundleInfo(**bundle)

vm_bundle = rule(
    doc = """\
Packages one or more binaries into a bundle suitable for use from an emulator.

This is only needed if you need to provide a specific argv, or to
supply a script to prepare the environment to run the VM, or to clean
up afterward.

The attributes are executed in the order specified: "prepare", then "init", 
"run", "check", and "cleanup".

The "prepare", "check", and "cleanup" steps are run OUTSIDE the VM.
The "init" and "run" steps are run INSIDE the VM.

When in debugging mode, a shell is provided after the "init" step is run,
but before the "run" step is run.
The "check" step is only run if the "run" step succeeds.

As an example, let's say you need to create an "internal-test" that:
  1) runs some setup commands outside the VM ("parepare" step)
  2) has to do something in the VM to prepare it
     (for example, tune the kernel - "init" step)
  3) runs a test binary in the VM ("run" step)
  4) runs some cleanup commands outside the VM ("cleanup" step)
  5) checks the result of the test at the end of the run ("check" step)

Now, not all those steps have to be run every time:

  1) If the user wants to have a shell in the VM and debug it,
     the init step should be run, but not the run one.
  2) If the run (or init) commands fail, there is no point in
     running the check command - we know it failed already!

To create "internal-test", you could use vm_bundle like this:

    sh_binary(
        name = "prepare-environment-outside-vm",
        srcs = [ ... a shell script ... ],
        data = [ ... a bunch of static files ...],
    )

    sh_binary(
        name = "init-binary-inside-vm",
        srcs = [ ... a shell script ... ],
        data = [ ... a bunch of static files ...],
    )

    sh_binary(
        name = "test-binary-inside-vm",
        srcs = [ ... a shell script ... ],
        data = [ ... a bunch of static files ...],
    )

    sh_binary(
        name = "check-results-outside-vm",
        srcs = [ ... a shell script ... ],
        data = [ ... a bunch of static files ...],
    )

    vm_bundle(
        name = "internal-test"
        prepare_cmds = [
            "cp /etc/hosts $OUTPUT_DIR",
        ],
        prepare_bin = ":prepare-environment-outside-vm",
        prepare_args = "--read --aggressive",

        init_bin = ":init-binary-inside-vm",

        run_bin = ":test-binary-inside-vm",
        
        cleanup_cmds = [
            "# cleanup goes here",
        ]

        check_cmds = [
            "jq $OUTPUT_DIR > $OUTPUT_DIR/file.json",
        ],
        check_bin = ":check-results-outside-vm",
        check_args = "--check $OUTPUT_DIR/file.json",
    )
""",
    implementation = _vm_bundle,
    attrs = {
        "prepare": attr.label_list(
            doc = "List of binaries or bundles to run OUTSIDE THE VM to prepare the environment (cannot be combined with prepare_bin/_args - optional)",
            cfg = "exec",
        ),
        "prepare_cmds": attr.string_list(
            doc = "Shell commands to run OUTSIDE THE VM to prepare the environment (before prepare_bin - optional)",
        ),
        "prepare_bin": attr.label(
            doc = "Binary to run OUTSIDE THE VM to prepare the environment (optional)",
            executable = True,
            cfg = "exec",
        ),
        "prepare_args": attr.string(
            doc = "Optional parameters to pass to the prepare_bin. Can use shell expansion.",
        ),
        "init": attr.label_list(
            doc = "List of binaries or bundles to run INSIDE THE VM to prepare the VM (cannot be combined with init_bin/_args - optional)",
            cfg = "target",
        ),
        "init_cmds": attr.string_list(
            doc = "Shell commands to run INSIDE THE VM to prepare the VM (before init_bin - optional)",
        ),
        "init_bin": attr.label(
            doc = "Binary to run INSIDE THE VM to prepare the VM (optional)",
            executable = True,
            cfg = "target",
        ),
        "init_args": attr.string(
            doc = "Optional parameters to pass to the init_bin. Can use shell expansion.",
        ),
        "run": attr.label_list(
            doc = "List of binaries to run INSIDE THE VM (cannot be combined with init_bin/_args - optional)",
            cfg = "target",
        ),
        "run_cmds": attr.string_list(
            doc = "Shell commands to run INSIDE THE VM (after init.*, before run_bin - optional)",
        ),
        "run_bin": attr.label(
            doc = "Binary to run INSIDE THE VM to run the command (after init.*, after run_cmds, optional)",
            executable = True,
            cfg = "target",
        ),
        "run_args": attr.string(
            doc = "Optional parameters to pass to the run_bin. Can use shell expansion.",
        ),
        "cleanup": attr.label_list(
            doc = "List of binaries to run OUTSIDE the VM AFTER the RUN to clean up the environment (cannot be combined with cleanup_bin/_args - optional)",
            cfg = "exec",
        ),
        "cleanup_cmds": attr.string_list(
            doc = "Shell commands to run OUTSIDE the VM AFTER the RUN to clean up the environment (before cleanup_bin - optional)",
        ),
        "cleanup_bin": attr.label(
            doc = "Binary to run OUTSIDE the VM AFTER the RUN (in reverse order) to clean up the environment (optional)",
            executable = True,
            cfg = "exec",
        ),
        "cleanup_args": attr.string(
            doc = "Optional parameters to pass to the cleanup_bin. Can use shell expansion.",
        ),
        "check": attr.label_list(
            doc = "List of binaries to run OUTSIDE the VM to check the environment (cannot be combined with check_bin/_args - optional)",
            cfg = "exec",
        ),
        "check_cmds": attr.string_list(
            doc = "Shell commands to check OUTSIDE THE VM to check the environment (before check_bin - optional)",
        ),
        "check_bin": attr.label(
            doc = "Binary to check OUTSIDE THE VM to check the environment (optional)",
            executable = True,
            cfg = "exec",
        ),
        "check_args": attr.string(
            doc = "Optional parameters to pass to the check_bin. Can use shell expansion.",
        ),
    },
)

def _kunit_bundle(ctx):
    ki = ctx.attr.image[KernelImageInfo]
    mods = get_compatible(ctx, ki.arch, ki.package, ctx.attr.module)
    alldeps = expand_deps(ctx, mods, ctx.attr.depth)

    commands = []
    inputs = []
    for kmod in alldeps:
        commands += ["", "# module " + package(kmod.label)]
        if kmod.setup:
            commands += kmod.setup

        for mod in kmod.files:
            if mod.extension != "ko":
                continue
            commands.append("load " + mod.short_path)
            inputs.append(mod)

    init = ctx.actions.declare_file(ctx.attr.name + "-kunit.sh")
    ctx.actions.expand_template(
        template = ctx.file.template_kunit,
        output = init,
        substitutions = {
            "{target}": package(ctx.label),
            "{message}": "KUNIT TESTS",
            "{commands}": "\n".join(commands),
        },
        is_executable = True,
    )

    check = ctx.actions.declare_file(ctx.attr.name + "-check.sh")
    ctx.actions.expand_template(
        template = ctx.file.template_check,
        output = check,
        substitutions = {
            "{target}": package(ctx.label),
            "{parser}": ctx.executable._parser.short_path,
        },
        is_executable = True,
    )
    outside_runfiles = ctx.runfiles(files = ctx.attr._parser.files.to_list())
    outside_runfiles = outside_runfiles.merge(ctx.attr._parser.default_runfiles)
    inside_runfiles = ctx.runfiles(inputs)

    return [
        DefaultInfo(files = depset([init, check]), runfiles = inside_runfiles.merge(outside_runfiles)),
        RuntimeBundleInfo(
            run = [RuntimeInfo(origin = ctx.label, binary = init, runfiles = inside_runfiles)],
            check = [RuntimeInfo(origin = ctx.label, binary = check, runfiles = outside_runfiles)],
        ),
    ]

kunit_bundle = rule(
    doc = """\
Generates a directory containing the kernel modules, all their dependencies,
and an init script to run them as a kunit test.""",
    implementation = _kunit_bundle,
    attrs = {
        "module": attr.label(
            mandatory = True,
            providers = [KernelBundleInfo],
            doc = "The label of the KUnit linux kernel module to be used for testing. It must define a kunit_test_suite so that when loaded, KUnit will start executing its tests.",
        ),
        "image": attr.label(
            mandatory = True,
            providers = [KernelImageInfo],
            doc = "The label of a kernel image this test will run against. Important to select the correct architecture and package module.",
        ),
        "depth": attr.int(
            default = 5,
            doc = "Maximum recursive depth when expanding a list of kernel module dependencies.",
        ),
        "template_kunit": attr.label(
            allow_single_file = True,
            default = Label("//bazel/linux:templates/kunit.template.sh"),
            doc = "The template to generate the bash script used to run the tests.",
        ),
        "template_check": attr.label(
            allow_single_file = True,
            default = Label("//bazel/linux:templates/check_kunit.template.sh"),
            doc = "The template to generate the bash script used to run the tests.",
        ),
        "_parser": attr.label(
            default = Label("//bazel/linux/kunit:kunit_zip"),
            doc = "KUnit TAP output parser.",
            executable = True,
            cfg = "exec",
        ),
    },
)
