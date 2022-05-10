# Generating a rootfs image with Buildroot

The `buildroot/` directory contains files that together with `buildroot` 2020.11.2 release (should work for newer releases as well) can generate a slim 60MiB image, suitable for executing KUnit tests defined through the *kernel_test* bazel rule.

In the directory, you can find:
* A `.config` file for buildroot, with a sane default configuration for the kernel testing use case
* A `kunit_post_build.sh` script. Buildroot will run this script after generating the rootfs image to:
  * Modify the default getty prompt to a simple shell, bypassing the login prompt
  * Add a new startup script (*run_kunit_tests.sh*)
* A `run_kunit_tests.sh` script which:
  * Mounts the hostfs
  * Loads all kernel modules available there (triggering kunit tests execution)

# Workflow to generate a custom rootfs image
1. Download buildroot release 2020.11.2 (or a newer one if you prefer)
   * `wget https://github.com/buildroot/buildroot/archive/refs/tags/2020.02.12.tar.gz`
2. Copy the content of the `buildroot/` directory in the dir you unpacked buildroot:
   * `rsync -avz ./buildroot/ buildroot-2022.02.1/` (make sure the .config file is copied)
3. Optional: modify the default buildroot config
   * `make menuconfig # Modify the default provided by the patch`
4. Generate the rootfs image
   * `make`
5. The image can be found in `output/images`

## Editing or tweaking the image

To make simple changes to the image:

1. `enkit astore get $uid` or `enkit astore get $path` to download the image.
2. `apt-get install fuse2fs` if not already installed on your machine.
3. `fuse2fs ./rootfs.img /mnt/tmp`
4. Edit edit edit 
5. `sync` and `umount /mnt/tmp` or `fusermount -u /mnt/tmp` (IMPORTANT, otherwise you corrupt the image)
6. And then push as a new image to astore. If you do so, make sure to use:
   `enkit astore annotate $uid "message message message"` to annotate the image with a message.
