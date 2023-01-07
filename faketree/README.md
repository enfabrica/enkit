# What is faketree?

`faketree` is a simple command line tool that uses linux namespaces
to provide a shell running on a filesystem with a different directory
layout.

You can read more about faketree in [our blog post](https://blog.enfabrica.net/different-file-system-views-for-different-tools-a425f13bb7f0).

Compared to other tools (like nsroot, unshare, fakeroot, fakechroot, sandboxfs, ...),
`faketree`:

* **Does NOT require root**, any user can run it as long as pid and user namespaces
  are enabled on the system (and they are by default on most modern linux distros,
  you can use the included test to verify).
* **Runs commands with the same UID/GID of the user**, does NOT create a shell
  that appears to run as root (can be overridden by flags)
* **Does NOT require `LD_PRELOAD`, ptrace, or similar tricks**. It works with static
  binaries as well as with binaries that require a modified `LD_LIBRARY_PATH` or
  `LD_PRELOAD`, or have protections to prevent tracing.
* **Affects only the commands run within faketree**, which means you can run multiple
  instances of faketree in parallel by the same user on the same system and with different parameters.
* **Propagates environment variables and privileges correctly**, allowing even graphical
  tools to run correctly within faketree.
* **Does NOT rely on FUSE**, disk performance and performance are unaffected.
* **Can override individual files**, including `/proc` and `/sys` files.
* **Tries to handle signals correctly**, so integration in a CI/CD pipeline should be
  straightforward. Sending SIGTERM to faketree will propagate to the children, and
  faketree will correctly wait for all children and descendants to terminate before
  exiting. A `kill -9` of faketree will guarantee all descendants being killed as well.
* If the wrapped command calls realpath or checks the filesystem,
  the command will see just a normal file system with mounts.


![terminal showing use](docs/terminal.gif?raw=true "Example Terminal Session with faketree")

# Examples

    $ faketree --mount /etc:/tmp/myroot/etc --chdir /tmp/myroot

Will give you a shell running with your own UID in the directory
`/tmp/myroot`. Within this shell and this shell only, the content of
`/etc` will have been replaced by the content of `/tmp/myroot/etc`.

    $ faketree --mount /var:/tmp/myroot/var \
               --mount /etc:/tmp/myroot/etc --chdir /tmp/myroot

Same as above, but overriding multiple directories.

    $ faketree --mount /var:/tmp/myroot/var \
               --mount /etc:/tmp/myroot/etc --chdir /tmp/myroot -- ls

Same as above, but instead of giving you a shell, it will run the
ls command and show the output for you. The use of `--` is important,
as it instructs faketree that any other option past `--` is for `ls`,
for the command being run.

    $ echo "builder00" > proc_sys_kernel_hostname
    $ echo "127.0.0.1  builder00" > etc_hosts
    $ faketree --mount etc_hosts:/etc/hosts \
               --proc --mount :/proc:type=proc \
               --mount proc_sys_kernel_hostname:/proc/sys/kernel/hostname \
               --hostname not-a-builder -- bash
    $ cat /proc/sys/kernel/hostname
    builder00
    $ hostname
    not-a-builder

Will run the bash command in an environment where (in order of flags):

  * `/etc/hsots` has been replaced by the file we created named `etc_hosts`
    (shows a single file override)
  * `/proc/sys/kernel/hostname` has been replaced by the file `proc_sys_kernel_hostname
    (shows a proc file override). Note that `--proc --mount :/proc:type=proc` is
    key here: by default faketree will set up `/proc` **last**, as `faketree` internally
    needs `/proc`, and mounts it last to prevent **accidental** overrides.
    By using `--proc`, you instruct faketree that the flags take care of `proc` already.
    By using `--mount :/proc:type=proc` you are mounting proc right then and there.
    The empty string before the semicolon is equivalent to "none" in traditional mount,
    while the string after the last semicolon are mount options (most mount options are
    supported)
  * The shell believes the hostname is called 'not-a-builder' (remember: there's a
    syscall independent of `/proc` in most linux system to return the hostname,
    `--hostname` allows to override that value.

**More examples** are available in the [faketree_test.sh file](https://github.com/enfabrica/enkit/blob/master/faketree/faketree_test.sh),
complete with expected outputs and behaviors.

# Installation

Installing on your system should be as simple as:

    go install -v github.com/enfabrica/enkit/faketree@latest

At time of writing, `go` version 1.19 is required.

Alternatively, you can use our preferred build system (will work regardless of the go version on your system):

  1. Install `bazelisk` - [instructions here](https://docs.bazel.build/versions/5.1.1/install-bazelisk.html) or
     [here](https://github.com/bazelbuild/bazelisk/blob/master/README.md)

  2. Clone this repository:

            git clone https://github.com/enfabrica/enkit 

  3. Build faketree with:

            bazelisk build //faketree:faketree

  4. The faketree binary will now be ready for use in the directory `bazel-bin/faketree/faketree_/faketree`.
     It's a static binary, you should be able to just copy it and use it on any system.

# Testing

To test that `faketree` works correctly on your system, the easiest way is to install bazelisk and
clone the repository as shown above. You can then run:

    bazelisk test //faketree/...:all

To run all the faketree tests. You should see a list of PASSED/FAILED results then.

If you use bazel in a remote build environment, you can run the test in RBE to verify that
your remote build machines are configured to allow for nested namespaces, user and mount
namespaces to allow faketree to work.
