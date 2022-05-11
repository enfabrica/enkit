load("//bazel/linux:providers.bzl", "KernelBundleInfo", "KernelImageInfo", "KernelModulesInfo", "KernelTreeInfo", "RootfsImageInfo", "RuntimeBundleInfo")
load("//bazel/utils:messaging.bzl", "location", "package")

def is_module(dep):
    """Returns True if this dependency is considered a kernel module."""
    return KernelModulesInfo in dep or KernelBundleInfo in dep

def is_compatible(arch, pkg, dep):
    """Checks if the supplied KernelModulesInfo matches the parameters."""
    if dep.package == pkg and dep.arch == arch:
        return [dep]
    return []

def get_compatible(ctx, arch, pkg, dep):
    """Extracts the set of compatible dependencies from a single dependency.

    The same kernel module can be built against multiple kernel trees and
    architectures. When that happens, it is packed in a KernelBundleInfo.

    When a build has a dependency on a KernelBundleInfo or even a plain
    kernel module through KernelModulesInfo, the build requires finding the
    specific .ko .symvers files for linking - the ones matching the same kernel
    and arch being built with the current target.

    Given a dependency dep, either a KernelBundleInfo or a KernelModuleInfo,
    this macro returns the set of KernelModuleInfo to build against, or
    an error.

    Args:
      ctx: a bazel rule ctx, for error printing.
      arch: string, desired architecture.
      pkg: string, desired kernel package.
      dep: a KernelModuleInfo or KernelBundleInfo to check.

    Returns:
      List of KernelModuleInfo to link against.
    """
    mods = []
    builtfor = []
    if KernelModulesInfo in dep:
        di = dep[KernelModulesInfo]
        mods.extend(is_compatible(arch, pkg, di))
        builtfor.append((di.package, di.arch))

    if KernelBundleInfo in dep:
        for module in dep[KernelBundleInfo].modules:
            mods.extend(is_compatible(arch, pkg, module))
            builtfor.append((module.package, module.arch))

    # Confirm that the kernel test module is compatible with the precompiled linux kernel executable image.
    if not mods:
        fail("\n\nERROR: " + location(ctx) + "requires {module} to be built for\n  kernel:{kernel} arch:{arch}\nBut it is only configured for:\n  {built}".format(
            arch = arch,
            kernel = pkg,
            module = package(dep.label),
            built = "\n  ".join(["kernel:{pkg} arch:{arch}".format(pkg = pkg, arch = arch) for pkg, arch in builtfor]),
        ))

    return mods

def expand_deps(ctx, mods, depth):
    """Recursively expands the dependencies of a module.

    Args:
      ctx: a Bazel ctx object, used to print useful error messages.
      mods: list of KernelModulesInfo, modules to compute the dependencies of.
      depth: int, how deep to go in recursively expanding the list of modules.

    Returns:
      List of KernelModulesInfo de-duplicated, in an order where insmod
      as a chance to successfully resolve the dependencies.
    """
    error = """ERROR:

While recurisvely expanding the chain of 'kdeps' for the module,
at {depth} iterations there were still dependencies to expand.
Loop? Or adjust the 'depth = ' attribute setting the max.

The modules computed to load so far are:
  {expanded}

The modules still to expand are:
  {current}
"""
    alldeps = list(mods)
    current = list(mods)

    for it in range(depth):
        pending = []
        for mi in current:
            for kdep in mi.kdeps:
                pending.append(kdep)

        alldeps.extend(pending)
        current = pending

        # Error out if we still have not expanded the full set of
        # dependencies at the last iteration.
        if current and it >= depth - 1:
            fail(location(ctx) + error.format(
                depth = depth,
                expanded = [package(d.label) for d in alldeps],
                current = [package(d.label) for d in current],
            ))

    # alldeps here lists all the recurisve dependencies of the supplied
    # mods starting from the module themselves.
    #
    # But... kernel module loading requires having the dependencies loaded
    # first. insmod may also fail if a module is loaded twice.
    #
    # So: reverse the list, remove duplicates.
    dups = {}
    result = []
    for mod in reversed(alldeps):
        key = package(mod.label)
        if key in dups:
            continue
        result.append(mod)

    return result
