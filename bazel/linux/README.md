# Description
Bazel hooks for compiling and testing linux kernel modules against a specific kernel version.

# Main rules made available
`kernel_tree_version`* defines a specific kernel tree to compile and test modules against.
It must point to a .tar.gz with a specific format, most likely built using either [kbuild](https://github.com/enfabrica/enkit/tree/master/kbuild) or [generate_custom_archive.sh](https://github.com/enfabrica/enkit/blob/master/bazel/linux/run_um_kunit_tests.sh).

`kernel_image_version`* defines an executable kernel image. Currently, this is only required by the *kernel_test* rule to provide a user-mode linux executable image (which is used to launch a kernel local testing environment).
The *package* attribute must be the same declared by the `kernel_tree_version` relative to the kernel tree used to build this executable image.

`rootfs_version`* defines a root filesystem image, required to launch a user-mode linux executable image. This can be built, for example, with something like buildroot.

`kernel_module` defines a kernel module. It requires a *kernel_tree_version* rule label as a kernel parameter.
It generates a .ko file that can then be loaded in any machine running a kernel compatible with the one specified by *kernel_tree_version*.

`kernel_test` defines a kernel test. It requires a *kernel_module* rule label as a module parameter, representing the kernel module to be tested.
This has two other dependencies: a root filesystem image and executable user-mode linux kernel image (compatible with the kernel version used to compile the module to be tested).
When executed using *bazel test*, it will launch the user-mode linux image using the provided rootfs image and exposing the module to be tested through hostfs. For more info see [run_um_kunit_tests.sh](https://github.com/enfabrica/enkit/blob/master/bazel/linux/run_um_kunit_tests.sh).

\* *These are repository rules, they are usually added to a project WORKSPACE file, and can then be referenced using the @rule_name syntax.*

# Workflow
## Building a kernel module
1. Generate a .tar.gz using either [kbuild](https://github.com/enfabrica/enkit/tree/master/kbuild) or the [generate_custom_archive.sh](https://github.com/enfabrica/enkit/tree/master/kbuild/utils) script
2. Make the .tar.gz available through some https mirror at $URL
3. Add a `kernel_tree_version(name="my-kernel-tree", url="$URL", ...)` rule in your repository WORKSPACE file
4. Add a `kernel_module(name="my_module", kernel="my-kernel-tree", ...)` rule inside your BUILD.bazel file in your kernel module directory

## Testing a kernel module
1. Declare how to build the test kernel module following the instructions above
2. Add a `rootfs_version(name="my-rootfs", ulr="$URL", ...)` rule in your repository WORKSPACE file
3. Add a `kernel_image_version(name="my-kernel-image", ulr="$URL", ...)` rule in your repository WORKSPACE file
   * Check out the *generate_custom_archive.sh* [instructions](https://github.com/enfabrica/enkit/blob/master/kbuild/utils/README.md) if you don't know how to generate a suitable  user-mode linux executable image
4. Add a `kernel_test(name="my_module_test", module=":my_module", kernel_image="@my-kernel-image", rootfs_image="@my-rootfs")` rule inside your BUILD.bazel file in your kernel module directory
5. Test the kernel running `bazel test :my_module_test`
   * Use `--test_output=all` if you want to see the user-mode linux and kunit output

**NOTE**: currently we only support KUnit tests, i.e. your kernel module must define a *kunit_suite_test* as documented [here](https://kunit.dev/third_party/kernel/docs/start.html#writing-your-first-test).
Moreover, it is expected that the rootfs image you provide will take care of:
1. Mounting the exposed [hostfs](https://www.kernel.org/doc/html/latest/virt/uml/user_mode_linux_howto_v2.html?highlight=user%20mode%20linux#host-file-access)
2. Loading the test module .ko file made available through the hostfs directory.
This will trigger the KUnit tests execution, and the *kernel_test* rule will take care of parsing the tests output and communicating the results to bazel
