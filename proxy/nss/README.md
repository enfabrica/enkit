# NSS Autouser

This directory contains `libnss_autouser`. `libnss_autouser` can do
two separate things:

* Change user login configurations on the fly based on the user and
  the program the user is logging in with (dynamic user configuration).

  This is useful, for example, to throw users in different containers
  at login, or access the host/hypervisor.

* Help create users on the fly when password authentication is not used
  (dynamic user creation).

This module was designed to be used with `ssh` certificate based authentication
or `oauth` or `google` based authentication without having NIS, LDAP, or other
shared databases in the network.

If you don't know about ssh certificates, [this is a good article](https://berndbausch.medium.com/ssh-certificates-a45bdcdfac39)
to read, but please keep in mind that "ssh certificates" != "ssh keys".

## The big picture

When `sshd` (or `login`, or a graphic authentication program) is authenticating
a user, typicailly:

1. The daemon will look up the user, in the `passwd` database.

   This ends up invoking `getpwent`, which reads `/etc/nsswitch.conf`, which
   typically ends up reading the `/etc/passwd` file. But this can be configured
   by adding _moudles_ in `/etc/nsswitch.conf`, to, for example, use an ldap
   database.

2. Once the user is identified as existing (there is a passwd entry),
   by looking at this entry the daemon will know the home directory, if
   a password is accepted, the shell, the UID and GID.

3. `sshd` will use this information to check the home directory for the user
   for configuration files like the `authorized_keys` file, or apply policies
   defined in the `sshd_config` file, like `AllowUsers`, `DenyUsers`.

4. Assuming the user is allowed to log in, at this point the daemon
   will typically use some mechanism to authenticate the user by eg,
   verifying the user has the correct keys or certificates, or invoke
   the `PAM` library for authentication.

   The `PAM` library is configured via files in `/etc/pam.d/`, `sshd`
   for example, and provides a very flexible mechanism to pick and
   configure not only authentication methods, but actions that need
   to be performed to set up the user environment.

   For example, environment variables, memory or file descriptor limits,
   auditing tools, ...

5. If the user is authenticated, `PAM` is invoked once again to
   set up the user environment, and a `shell` is finally spawned.

## Installation

Regardless of the configuration, to install `libnss_autouser` you need to:

1. Build `libnss_autouser`, `bazelisk build //proxy/nss:libnss_autouser`
   in the top level directory of this repository, and look for the path
   of the output file in the bazel output.

2. Copy `libnss_autouser.so` in the correct path, generally (*important*:
   name has to end with `.so.2` in the final path):

       cp -f bazel-bin/proxy/nss/libnss_autouser.so /lib/x86_64-linux-gnu/libnss_autouser.so.2

3. Edit `/etc/nsswitch.conf` to have the correct line, something like:

       passwd:         files autouser systemd

4. Create a `/etc/nss-autouser.conf` file, make sure it is set with `chmod 0600`.

The `configs` directory contains a couple examples, but read along.


## Dynamic User Creation

Let's say you are using `ssh` certificates, and you are managing a cluster
full of machines.

When a user presents its certificates, by checking the signature on the key
`sshd` can be certain of the identity of the user. The certificate contains
both the authorized identities, and a signature by a trusted authority.

On a typical linux setup, the user would still *be denied entry*: as the user
does not exist in the `passwd` database, unless something like LDAP or NIS is used.

`libnss_autouser` allows to:

1. Assign a free UID, GID, and home on the fly based on the user name.
   When `libnss_autouser` runs, it exports a few environment variables
   indicating the details of the user.

2. If `sshd` then authenticates the user, `libpam_script` can then be used
   before the login completes to create the home directory, and add the
   user to the `passwd` file.

To do so, you need to:

1. Install `libnss_autouser` as described above.

2. Create a `/etc/nss-autouser.conf` file, make sure it is set with `chmod 0600`,
   with something like:

       Seed enfabrica-test # change the seed to a random value that you like!
       # DebugLog /tmp/debug.log
       
       Match sshd:*
         MinUid 70000
         MaxUid 0xfffffff0
         Shell /bin/bash

3. Test that nothing has broken, `getent passwd root` or `getent passwd non-existing-user`
   should lead the correct result (no crash, no error, ...). If you change the `Match` line
   to have `getent`, you should see `getent passwd non-existing-user` start succeeding
   with a phony home, uid and gid.

4. Configure `libpam-script` on your system to run a script to create the user on the fly.
   This generally requires installing `libpam-script` (`apt-get install libpam-script`) and...

5. Edit the `/etc/pam.d/sshd` file to have the line:

       account required  pam_script.so dir=/etc/security

6. Create a script `/etc/security/pam_script_acct` (don't forget to `chmod 0755` it)
   to create the user on the fly.

And enjoy!

You can see an example `pam_script_acct` and `/etc/pam.d/sshd` configuration file in
`./configs`.

If this does not work, read the [debugging](#Debugging) section.


## Dynamic User Configuration

Let's that you use docker containers (or VMs) to provide shells to your users.
Or let's say that you have a shared home directory on NFS, that sometimes fails.

With `libnss_autouser` you can configure pattern matching so for example:

* When an user `alice` logs into a system as `alice-nonfs`, a different `/home`
  directory is used.

* When an user `bob` logs into a system as `alice-host`, instead of running a
  shell that throws the user in a container, a shell that gives access to the
  host is used.

To use this:

1. Build and install `libnss_autouser` as described above.

2. Create a `/etc/nss-autouser.conf` file, make sure it is set with `chmod 0600`,
   with something like:

       Seed enfabrica-test # change the seed to a random value that you like!
       # DebugLog /tmp/debug.log
       
       Match sshd:*
         MinUid 70000       # Very important!!
         MaxUid 0xfffffff0  # Very important!!

         Shell /bin/run-docker

         Suffix -nonfs
           Home /usr/local/home

         Suffix -host
           Shell /bin/bash


Make sure to specify a `MinUid` and `MaxUid` that matches the
uids assigned to your users. If a user has an uid outside this
range, no mangling will be performed.

# Security

The accounts generated on the fly by `libnss_autouser` have password
disabled by default, which on a properly configured system means that
the user cannot login without an alternative mechanism of authentication.

PAM plays an important role: if you disable ssh authentication and pam
authentication, the user won't even need an account to log onto the machine!

By specifying the MinUid and MaxUid range, you protect system accounts.

`libnss_autouser` is inherently racy: the UID and GID assigned are to all
effects effimeral and can be assigned to other users until they are saved in
the passwd database, until after the user successfully authenticates.

If *two non-existing users* with *valid credentials* and a *name that clashes
on the same computed hash* try to log in at the same time, they will be
assigned the same UID and GID.

However, when the `pam_script_acct` script is run, if written properly,
one of the users will succeed whlie the other user will be kicked out.

If this second user retries, he/she will be assigned a different UID/GID
(the module is guaranteed to assign a free UID/GID).

To ensure security, it is recommended to:

1. Make sure that `pam_script_acct` fails (exits with non 0 status) if
   the user already exists at time of insertion in the database. 

2. Make sure that whatever tool `pam_script_acct` uses to create the
   user does locking. Without locking, adding two users at the same
   time can corrupt the `passwd` database.

3. Make sure `pam_script.so` has the magic *required* word next to
   it, *appears first* in the configuration file (just to be safe),
   does not have `onerr=success`. Having a `pam_deny.so` right after
   this line may also be a good defense.

4. Configure `ssh` to `UsePAM yes` explicitly.

5. Do something so that your distro updates don't mess up your
   configuration files. A favourite of mine is to run `chattr +i`
   on critical files.

6. Make sure `/etc/nss-autouser.conf` is only writable by root,
   and that it is enabled only for very specific processes.
   Don't specify *MinUid* and *MaxUid* globally, but only inside
   *Match* statements.

Why not reserve UIDs immediately when `libnss_autouser` is invoked?

When the library is called, the user *has not* authenticated yet.
It would be trivial to DoS the system, or create millions of invalid users.

We could implement a small pool of recently used UIDs that cannot be
re-used to mitigate the risk, but this will still not prevent malicious
attempts.

Note that for the attack to be successful and lead to different users
having the same UID, *both users still need to have valid credentials*,
with the system using unsafe login scripts.


# Debugging

For debugging, here are a few suggestions:

1. Change the configuration file `/etc/nss-autouser.conf` to have a `DebugLog` file.
   Look at the content of the log file.

2. Check `/var/log/syslog` or `journalctl`. `libnss_autouser` tries to log problems.

3. Change `/etc/nss-autouser.conf` to `Match` on `getent`, and use `strace getent 2>&1 |less`
   to check what is happening, search for the string `autouser`.
