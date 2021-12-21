# What is faketree?

`faketree` is a simple command line tool that uses linux namespaces
to provide a shell running on a filesystem with a different directory
layout.

Compared to other tools (like nsroot, unshare, fakeroot, fakechroot, sandboxfs, ...),
`faketree`:

* Does NOT require root to run, can run as any user.
* Does NOT create a shell that appears to run as root (by default
  the shell will see the same UID/GID as the running user).
* Does NOT require `LD_PRELOAD` or similar tricks, that do not
  work for static binaries.
* Affects only the commands run within faketree. 
* If the wrapped command calls realpath or checks the filesystem,
  the command will see just a normal file system with mounts.

* Does require support for user namespaces to be enabled.

# Examples

    $ faketree --mount /etc:/tmp/myroot/etc --chdir /tmp/myroot

Will give you a shell running with your own UID in the directory
/tmp/myroot. Within your root you will see a subdirectory etc,
that was mouunted from the original /etc.

    $ faketree --mount /var:/tmp/myroot/var \
               --mount /etc:/tmp/myroot/etc --chdir /tmp/myroot

Same as above, but using multiple mounts.

    $ faketree --mount /var:/tmp/myroot/var \
               --mount /etc:/tmp/myroot/etc --chdir /tmp/myroot -- ls

Same as above, but instead of giving you a shell, it will run the
ls command and show the output for you.

# Installation

Usual `go get`, although we recommend the use of bazel for builds.
