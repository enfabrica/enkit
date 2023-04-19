load("//bazel/linux:providers.bzl", "KernelImageInfo", "RootfsImageInfo", "RuntimeBundleInfo", "RuntimeInfo")
load("//bazel/utils:messaging.bzl", "location", "package")
load("//bazel/utils:types.bzl", "escape_and_join")
load("//bazel/utils:files.bzl", "files_to_dir")
load("@bazel_skylib//lib:shell.bzl", "shell")

def create_runner_attrs(template_init_default):
    """Returns a dict of attributes common to all runners.""" 

    return {
        "kernel_image": attr.label(
            mandatory = True,
            providers = [DefaultInfo, KernelImageInfo],
            doc = "The kernel image that will be used to execute this test. A string like @stefano-s-favourite-kernel-image, referencing a kernel_image(name = 'stefano-s-favourite-kernel-image', ...",
        ),
        "rootfs_image": attr.label(
            mandatory = False,
            providers = [RootfsImageInfo],
            doc = """\
The rootfs image that will be used to execute this test.

A string like @stefano-s-favourite-rootfs-image, referencing a rootfs_image(name = 'stefano-s-favourite-rootfs-image', ...).
If not specified, the current root of the filesystem will be used as rootfs.
""",
        ),
        "run": attr.label_list(
            mandatory = True,
            doc = "List of executable targets to run in the emulator.",
        ),
        "wrapper_flags": attr.string_list(
            doc = """\
Flags to append after '--' at the end of the qemu command line.

This is useful when the emulator is being invoked through a wrapper, or
when a wrapper is invoked by the emulator. It allows to separate the
emulator flags from those passed to the wrapper.
""",
            default = [],
        ),
        "template_init": attr.label(
            allow_single_file = True,
            default = template_init_default,
            doc = "The template to generate the init script running in the VM.",
        ),
        "template_start": attr.label(
            allow_single_file = True,
            default = Label("//bazel/linux:templates/runner.template.sh"),
            doc = "The template to generate the bash script to run the emulator.",
        ),
    }

def commands_and_runtime(ctx, msg, runs):
    """Computes commands and runfiles from a list of RuntimeInfo"""
    commands = []
    runfiles = ctx.runfiles()
    labels = []
    for r, rbl in runs:
        for rbi in rbl:
            if not hasattr(rbi, "commands") and (not hasattr(rbi, "binary") or not rbi.binary):
                fail(location(ctx) + (" the '{msg}' step in {target} must be executable, " +
                                      "and have a binary defined, or provide commands to run").format(msg = msg, target = package(r.label)))

            if hasattr(rbi, "commands") and rbi.commands:
                if not getattr(rbi, "origin", False):
                    commands.append("echo '==== {msg}: {target} -- inline commands'".format(
                        msg = msg,
                        target = package(r.label),
                    ))
                    labels.append(str(r.label))
                commands.extend(rbi.commands)
                for command in rbi.commands:
                    labels.append("{label}:{cmd}".format(label = r.label, cmd = command))

            if hasattr(rbi, "binary") and rbi.binary:
                binary = rbi.binary
                args = ""
                if hasattr(rbi, "args"):
                    args = rbi.args

                if not getattr(rbi, "origin", False):
                    commands.append("echo '==== {msg}: {target} as \"{path} {args}\"...'".format(
                        msg = msg,
                        target = package(r.label),
                        path = rbi.binary.short_path,
                        args = args,
                    ))
                    labels.append(str(r.label))
                commands.append("{binary} {args}".format(
                    binary = shell.quote(binary.short_path),
                    args = args,
                ))
                labels.append(str(r.label))

                runfiles = runfiles.merge(ctx.runfiles([binary]))

            if hasattr(rbi, "runfiles") and rbi.runfiles:
                runfiles = runfiles.merge(rbi.runfiles)

    if len(labels) != len(commands):
        fail(location(ctx) +
             "enkit error: count mismatch between labels ({len_lab}) and commands ({len_cmd})".format(
                 len_lab = len(labels),
                 len_cmd = len(commands),
             ))

    return commands, runfiles, labels

def runtime_info_from_target(ctx, target, **kwargs):
    """Creates a RuntimeInfo provider from a binary target.

    This function extracts info from a DefaultInfo provider, and populates the
    corresponding RuntimeInfo fields.
    """
    info = dict(**kwargs)
    if target:
        di = target[DefaultInfo]
        if not di.files_to_run or not di.files_to_run.executable:
            fail(location(ctx) + (" asked to run target {target}, but " +
                                  "it is not executable! It does not generate a binary. Fix BUILD.bazel file?".format(
                                      target = package(target.label),
                                  )))
        info["binary"] = di.files_to_run.executable
        info["runfiles"] = di.default_runfiles

    return RuntimeInfo(**info)


def get_prepare_run_check(ctx, run):
    """Returns a [(label, [RuntimeInfo]), ...] for each bundle or bin in run.

    Args:
      ctx: a bazel context, used for debug/error messages.
      run: a list of executable targets or RuntimeBundleInfo objects.

    Returns:
      (prepares, inits, runs, checks, cleanups), where each of them is an array
      of (label, [RuntimeInfo]) defining what to run in each of those steps. 
    """
    prepares = []
    inits = []
    runs = []
    cleanups = []
    checks = []
    for r in run:
        if RuntimeBundleInfo in r:
            rbi = r[RuntimeBundleInfo]
            if hasattr(rbi, "prepare") and rbi.prepare:
                prepares.append((r, rbi.prepare))
            if hasattr(rbi, "init") and rbi.init:
                inits.append((r, rbi.init))
            if hasattr(rbi, "run") and rbi.run:
                runs.append((r, rbi.run))
            if hasattr(rbi, "cleanup") and rbi.cleanup:
                cleanups.append((r, rbi.cleanup))
            if hasattr(rbi, "check") and rbi.check:
                checks.append((r, rbi.check))
            continue

        runs.append((r, [runtime_info_from_target(ctx, r)]))

    cleanups = list(reversed(cleanups))
    return prepares, inits, runs, cleanups, checks

def expand_targets_and_bundles(ctx, attr, verbose = True):
    """Returns the commands to run for the binaries or bundles supplied.

    Args:
      ctx: a bazel context, used for error message purposes.
      attr: generally a label_list attribute, a list of targets that are either
        executable, or represent a bundle, with the RuntimeBundleInfo provider.

    Returns:
      A struct representing the commands to run for each phase and the
      required labels and runfiles.
    """
    prepares, inits, runs, cleanups, checks = get_prepare_run_check(ctx, attr)

    cprepares, rprepares, lprepares = commands_and_runtime(ctx, "prepare", prepares)
    cchecks, rchecks, lchecks = commands_and_runtime(ctx, "check", checks)
    ccleanups, rcleanups, lcleanups  = commands_and_runtime(ctx, "cleanup", cleanups)
    outside_runfiles = ctx.runfiles().merge_all([rprepares, rchecks, rcleanups])

    cinits, rinits, linits = commands_and_runtime(ctx, "init", inits)
    cruns, rruns, lruns = commands_and_runtime(ctx, "run", runs)
    inside_runfiles = ctx.runfiles().merge_all([rinits, rruns])

    return struct(
        inside_runfiles = inside_runfiles,
        outside_runfiles = outside_runfiles,
        commands = struct(
            prepare = cprepares,
            check = cchecks,
            cleanup = ccleanups,
            init = cinits,
            run = cruns,
        ),
        runfiles = struct(
            prepare = rprepares,
            check = rchecks,
            cleanup = rcleanups,
            init = rinits,
            run = rruns,
        ),
        labels = struct(
            prepare = lprepares,
            check = lchecks,
            cleanup = lcleanups,
            init = linits,
            run = lruns,
        ),
    )

def create_runner(ctx, archs, code, runfiles = None, extra = {}):
    ki = ctx.attr.kernel_image[KernelImageInfo]
    if archs and ki.arch not in archs:
        fail(
            location(ctx) + ("the kernel image '{name}' of architecture '{arch}' " +
                             "does not support the required architectures {archs}. Check 'arch = ...'").format(
                name = ki.name,
                arch = ki.arch,
                archs = archs,
            ),
        )

    torun = expand_targets_and_bundles(ctx, ctx.attr.run)

    init = ctx.actions.declare_file(ctx.attr.name + "-init.sh")
    ctx.actions.expand_template(
        template = ctx.file.template_init,
        output = init,
        substitutions = {
            "{message}": "INIT STARTED",
            "{target}": package(ctx.label),
            "{relpath}": init.short_path,
            "{inits}": "\n".join(torun.commands.init),
            "{commands}": "\n".join(torun.commands.run),
        },
        is_executable = True,
    )

    runtime_root = files_to_dir(
        ctx,
        ctx.attr.name + "-root",
        torun.inside_runfiles.files.to_list() + [init],
        post = "cd {dest}; cp -L %s ./init.sh" % (shell.quote(init.short_path)),
    )
    outside_runfiles = torun.outside_runfiles.merge(ctx.runfiles([runtime_root, ki.image]))
    if runfiles:
      outside_runfiles = outside_runfiles.merge(runfiles)

    rootfs = ""
    if ctx.attr.rootfs_image:
        rootfs = ctx.attr.rootfs_image[RootfsImageInfo].image.short_path
        inputs = depset(transitive = [inputs, ctx.attr.rootfs_image.files])

    subs = dict({
        "target": package(ctx.label),
        "prepares": "\n".join(torun.commands.prepare),
        "cleanups": "\n".join(torun.commands.cleanup),
        "checks": "\n".join(torun.commands.check),
        "kernel": ki.image.short_path,
        "rootfs": rootfs,
        "init": init.short_path,
        "runtime": runtime_root.short_path,
	"wrapper_flags": shell.array_literal(ctx.attr.wrapper_flags),
    }, **extra)

    subs["code"] = code.format(**subs)
    start = ctx.actions.declare_file(ctx.attr.name + "-start.sh")
    ctx.actions.expand_template(
        template = ctx.file.template_start,
        output = start,
        substitutions = dict([("{%s}" % (k), v) for k, v in subs.items()]),
        is_executable = True,
    )
    return [DefaultInfo(runfiles = outside_runfiles, executable = start)]
