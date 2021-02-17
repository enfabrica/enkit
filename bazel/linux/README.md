# Description
Bazel hooks for compiling and testing linux kernel modules against a specific kernel version.

# Main rules made available
`kernel_version` defines a specific kernel version to compile and test modules against.
It must point to a .tar.gz with a specific format, most likely built using either [kbuild](https://github.com/enfabrica/enkit/tree/master/kbuild) or [generate_custom_archive.sh](https://github.com/enfabrica/enkit/blob/master/bazel/linux/run_um_kunit_tests.sh).
It is usually added to a project WORKSPACE file, defining a repository which can then be referenced using the @rule_name syntax.

`kernel_module` defines a kernel module. It requires a *kernel_version* rule label as a kernel parameter.
It generates a .ko file that can then be loaded in any machine running a kernel compatible with the one specified by *kernel_version*.

`kernel_test` defines a kernel test. It requires a *kernel_module* rule label as a module parameter, representing the kernel module to be tested.
Additionally, it requires the labels of two files, one representing a root filesystem image, the other an executable user-mode linux kernel image (compatible with the kernel version used to compile the module to be tested).
When executed using *bazel test*, it will launch the user-mode linux image using the provided rootfs image and exposing the module to be tested through hostfs. For more info see [run_um_kunit_tests.sh](https://github.com/enfabrica/enkit/blob/master/bazel/linux/run_um_kunit_tests.sh)

# Workflow
## Building a kernel module
1. Generate a .tar.gz using either [kbuild](https://github.com/enfabrica/enkit/tree/master/kbuild) or the [generate_custom_archive.sh](https://github.com/enfabrica/enkit/tree/master/kbuild/utils) script
2. Make the .tar.gz available through some https mirror at $URL
3. Define a `kernel_version(name="my-kernel-version", url="$URL", ...)` rule in your repository WORKSPACE file
4. Add a `kernel_module(name="my-kernel-module", kernel="my-kernel-version", ...)` rule inside your BUILD.bazel file in your kernel module directory

## Testing a kernel module
1. Build a kernel module following the instructions above
2. Add an `http_file(name="rootfs-img", ...)` rule in your repository WORKSPACE file (same for the executable kernel image)
   * Check out the *generate_custom_archive.sh* [instructions](https://github.com/enfabrica/enkit/blob/master/kbuild/utils/README.md) if you don't know how to generate a suitable  user-mode linux executable image
4. Add a `kernel_test(name="my-kernel-module-test", module=":my-kernel-module", kernel_image="@kernel-img//file", rootfs_image="@rootfs-img//file")` rule inside your BUILD.bazel file in your kernel module directory
5. Test the kernel running `bazel test :my-kernel-module-test`

**NOTE**: currently we only support KUnit tests, i.e. your kernel module must define a *kunit_suite_test* as documented [here](https://kunit.dev/third_party/kernel/docs/start.html#writing-your-first-test).
Moreover, it is expected that the rootfs image you provide will take care of:
1. Mounting the exposed hostfs
2. Loading the test module .ko file made available through the hostfs directory.
This will trigger the KUnit tests execution, and the *kernel_test* rule will take care of parsing the tests output and communicating the results to bazel
