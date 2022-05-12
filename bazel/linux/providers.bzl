KernelTreeInfo = provider(
    doc = """Maintains the information necessary to build a module out of a kernel tree.

In a rule(), you will generally want to create a 'make' command using 'make ... -C $root/$build ...'.
Note that the kernel tree may depend on tools or ABIs not installed/available on your system,
a kernel_tree on its own is not expected to be hermetic.
""",
    fields = {
        "name": "Name of the rule that defined this kernel tree. For example, 'carlo-s-favourite-kernel'.",
        "package": "A string indicating which package this kernel is coming from. For example, 'centos-kernel-5.3.0-1'.",
        "root": "Bazel directory containing the root of the kernel tree. This is generally the location of the top level BUILD.bazel file. For example, external/@centos-kernel-5.3.0-1.",
        "build": "Relative path of subdirectory to enter to build a kernel module. It is generally the 'build' parameter passed to the kernel_tree rule. For example, lib/modules/centos-kernel-5.3.0-1/build.",
    },
)

KernelModulesInfo = provider(
    doc = """Maintains the information necessary to represent compiled kernel modules.""",
    fields = {
        "label": "The Label() defining this kernel module.",
        "package": "A string indicating which package this kernel module has been built against. For example, 'centos-kernel-5.3.0-1'.",
        "arch": "A string describing the architecture this module was built against.",
        "files": "A list of files representing the compiled .ko files part of this module.",
        "kdeps": "A list of other KernelModulesInfo Target() objects needed at run time to load this module.",
        "setup": "A list of strings, each string a shell command needed to prepare the system to load this module.",
    },
)

KernelBundleInfo = provider(
    doc = "Represents a set of the same module compiled for different kernels or arch.",
    fields = {
        "modules": "List of targets part of this bundle. Those targets provide a KernelModulesInfo.",
    },
)

KernelImageInfo = provider(
    doc = """Maintains the information necessary to represent a kernel executable image.""",
    fields = {
        "name": "Name of the rule that defined this kernel executable image. For example, 'stefano-s-favourite-kernel-image'.",
        "package": "A string indicating which package this kernel executable image is coming from. For example, 'custom-5.9.0-um'.",
        "arch": "Architecture this linux kernel image was built for.",
        "image": "Path of the kernel executable image.",
    },
)

RootfsImageInfo = provider(
    doc = """Maintains the information necessary to represent a rootfs image.

A rootfs is a file loadable by kvm/qemu/uml as a root file system. This root
file system is expected to be able to run a bash script as the init command,
and have basic tools available necessary for its users.
""",
    fields = {
        "name": "Name of the rule that defined this rootfs image. For example, 'stefano-s-favourite-rootfs'.",
        "image": "File containing the rootfs image.",
    },
)

RuntimeInfo = provider(
    doc = """Represents a binary to run""",
    fields = {
        "binary": "File object, executable, binary to run",
        "runfiles": "runfiles() object, representing the files needed by the binary at run time",
        "argv": "array of strings, optional arguments to pass to the binary",
    },
)

RuntimeBundleInfo = provider(
    doc = """Represents something to run in a VM environment.""",
    fields = {
        "prepare": "RuntimeInfo, executable (and its runfiles) to run OUTSIDE the VM BEFORE the RUN to prepare the environment",
        "run": "RuntimeInfo, executable (and its runfiles) to run INSIDE the VM",
        "check": "RuntimeInfo, executable (and its runfiles) to run OUTSIDE the VM AFTER the RUN to check if the run was successful",
    },
)
