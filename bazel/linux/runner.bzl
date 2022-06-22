load("//bazel/linux:providers.bzl", "KernelImageInfo", "RootfsImageInfo", "RuntimeBundleInfo", "RuntimeInfo")
load("//bazel/utils:messaging.bzl", "location", "package")
load("//bazel/utils:types.bzl", "escape_and_join")
load("//bazel/utils:files.bzl", "files_to_dir")
load("@bazel_skylib//lib:shell.bzl", "shell")

def create_runner_attrs(template_init_default):
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

def commands_and_runtime(ctx, msg, runs, runfiles, verbose = True):
    """Computes commands and runfiles from a list of RuntimeInfo"""
    commands = []
    runfiles = ctx.runfiles().merge(runfiles)
    labels = []
    for r, rbi in runs:
        if not hasattr(rbi, "commands") and (not hasattr(rbi, "binary") or not rbi.binary):
            fail(location(ctx) + (" the '{msg}' step in {target} must be executable, " +
                                  "and have a binary defined, or provide commands to run").format(msg = msg, target = package(r.label)))

        if hasattr(rbi, "commands") and rbi.commands:
            if verbose:
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

            if verbose:
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

def get_prepare_run_check(ctx, run):
    prepares = []
    runs = []
    checks = []
    for r in run:
        if RuntimeBundleInfo in r:
            rbi = r[RuntimeBundleInfo]
            if hasattr(rbi, "prepare") and rbi.prepare:
                prepares.append((r, rbi.prepare))
            if hasattr(rbi, "run") and rbi.run:
                runs.append((r, rbi.run))
            if hasattr(rbi, "check") and rbi.check:
                checks.append((r, rbi.check))
            continue

        di = r[DefaultInfo]
        if not di.files_to_run or not di.files_to_run.executable:
            fail(location(ctx) + (" run= attribute asks to run target {target}, but " +
                                  "it is not executable! It does not generate a binary. Fix BUILD.bazel file?".format(
                                      target = package(r.label),
                                  )))

        runs.append((r, RuntimeInfo(
            binary = di.files_to_run.executable,
            runfiles = di.default_runfiles,
        )))
    return prepares, runs, checks

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

    prepares, runs, checks = get_prepare_run_check(ctx, ctx.attr.run)

    outside_runfiles = ctx.runfiles()
    if runfiles:
        outside_runfiles = outside_runfiles.merge(runfiles)
    cprepares, outside_runfiles, _ = commands_and_runtime(ctx, "prepare", prepares, outside_runfiles)
    cchecks, outside_runfiles, _ = commands_and_runtime(ctx, "check", checks, outside_runfiles)
    cruns, inside_runfiles, _ = commands_and_runtime(ctx, "run", runs, ctx.runfiles())

    init = ctx.actions.declare_file(ctx.attr.name + "-init.sh")
    ctx.actions.expand_template(
        template = ctx.file.template_init,
        output = init,
        substitutions = {
            "{message}": "INIT STARTED",
            "{target}": package(ctx.label),
            "{relpath}": init.short_path,
            "{commands}": "\n".join(cruns),
        },
        is_executable = True,
    )

    runtime_root = files_to_dir(
        ctx,
        ctx.attr.name + "-root",
        inside_runfiles.files.to_list() + [init],
        post = "cd {dest}; cp -L %s ./init.sh" % (shell.quote(init.short_path)),
    )
    outside_runfiles = outside_runfiles.merge(ctx.runfiles([runtime_root, ki.image]))

    rootfs = ""
    if ctx.attr.rootfs_image:
        rootfs = ctx.attr.rootfs_image[RootfsImageInfo].image.short_path
        inputs = depset(transitive = [inputs, ctx.attr.rootfs_image.files])

    subs = dict({
        "target": package(ctx.label),
        "prepares": "\n".join(cprepares),
        "checks": "\n".join(cchecks),
        "kernel": ki.image.short_path,
        "rootfs": rootfs,
        "init": init.short_path,
        "runtime": runtime_root.short_path,
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
