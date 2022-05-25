load("//bazel/linux:uml.bzl", "kernel_uml_run")
load("//bazel/linux:qemu.bzl", "kernel_qemu_run")
load("//bazel/utils:macro.bzl", "mconfig", "mcreate_rule")
load("//bazel/utils:exec_test.bzl", "exec_test")
load("//bazel/linux:bundles.bzl", "kunit_bundle")

def qemu_test(name, kernel_image, run, qemu_binary = None,
              config = {}, **kwargs):
    """Instantiates all the rules necessary to create a qemu based test.

    Specifically:
        {name}-run: which when run will execute a kernel_qemu_run target
           with the configs specified in config.
        {name}: which when executed as a test will invoke {name}-run and
           succeed if the target exits with 0.

    Args:
        kernel_image, run, qemu_binary: passed as is to the generated
            kernel_qemu_run rule. Exposed externally for convenience.
        config: dict, all additional attributes to pass to the
            kernel_qemu_run rule, generally created with mconfig().
    """
    runner = mcreate_rule(
        name, kernel_qemu_run, "run", config, kwargs,
        mconfig(kernel_image = kernel_image, qemu_binary = qemu_binary),
    )
    exec_test(name = name, dep = runner, **kwargs)

def kunit_test(name, kernel_image, module, rootfs_image = None,
               kunit_bundle_cfg = {}, runner_cfg = {},
               runner = kernel_uml_run, **kwargs):
    """Instantiates all the rules necessary to create a kunit test.

    Creates 3 rules:
       {name}-runtime: which when built will create a kunit bundle for use.
       {name}-emulator: which when run will invoke the specified emulator
           together with the generated kunit runtime.
       {name}: which when executed as a test will invoke the emulator, and
           fail/succeed based on the results of the checks.
    Args:
      kernel_image: label, something like @type-of-kernel//:image,
          a kernel image to use.
      module: label, a module representing a kunit test to run.
      rootfs_image: optional, label, a rootfs image to use for the test.
      kunit_bundle_cfg: optional, dict, attributes to pass to the instantiated
          kunit_bundle rule, follows the mconfig use pattern.
      runner_cfg: optional, dict, attributes to pass to the instantiated
          runner rule, follows the mconfig use pattern.
      runner: a rule function, will be invoked to create the runner using
          the generated kunit bundle.
      kwargs: options common to all instantiated rules.
    """ 
    runtime = mcreate_rule(
        name,
        kunit_bundle,
        "runtime",
        kunit_bundle_cfg,
        kwargs,
        mconfig(module = module, image = kernel_image),
    )

    cfg = mconfig(run = [runtime], kernel_image = kernel_image)
    if rootfs_image:
        cfg = mconfig(cfg, rootfs_image = rootfs_image)
    name_runner = mcreate_rule(name, runner, "emulator", runner_cfg, kwargs, cfg)
    exec_test(name = name, dep = name_runner, **kwargs)
