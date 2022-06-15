load("@bazel_skylib//lib:paths.bzl", "paths")
load("@bazel_skylib//lib:shell.bzl", "shell")
load("@enkit//bazel/utils:types.bzl", "escape_and_join")

def _write_to_file_impl(ctx):
    ctx.actions.write(
        output = ctx.outputs.output,
        content = ctx.attr.content,
    )
    return [DefaultInfo(files = depset([ctx.outputs.output]))]

write_to_file = rule(
    doc = """
      Writes a string to a file.

      This method is mostly used in conjunction with diff_test to test bazel
      utility functions.
    """,
    implementation = _write_to_file_impl,
    attrs = {
        "output": attr.output(
            doc = "The file to write to.",
        ),
        "content": attr.string(
            doc = "The contents of the file.",
        ),
    },
)

def rebase_path(path, prefix, base):
    """Changes a path at a specified keyword to another path, appending the remaining onto it.

    Examples:
      >>> trim_path("/tree/foo/bar", "foo", "/etc")
      /etc/bar
    """
    s = path.split("/")
    tret = []
    flag = False
    for m in s:
        if m == prefix:
            flag = True
        if flag:
            tret.append(m)
    return paths.join(base, *tret)

def _rebase_and_copy_files_impl(ctx):
    all_input_files = ctx.files.source_files
    all_outputs = []
    for f in all_input_files:
        if ctx.attr.prefix:
            out_path = rebase_path(f.short_path, ctx.attr.prefix, ctx.attr.base_dir)
        else:
            out_path = paths.join(ctx.attr.base_dir, f.basename)
        out = ctx.actions.declare_file(out_path)
        all_outputs += [out]
        ctx.actions.symlink(
            output = out,
            target_file = f,
        )

    return [
        DefaultInfo(
            files = depset(all_outputs),
            runfiles = ctx.runfiles(files = all_outputs),
        ),
    ]

"""Copies all files in a directory to another directory, rebasing to a prefix as it goes along.

If you wish to copy straight into the directory, don't specify prefix.

Examples:
        >>> rebase_and_copy_files(
            source_files=[
                /etc/foo/bar.out,
                /etc/foo/curr/baz.out
            ],
            base_dir=/base,
            prefix=src
        )
        [
            /base/src/bar.out,
            /base/src/curr/bar.out,
        ]

        >>> rebase_and_copy_files(
            source_files=[
                /etc/foo/bar.out,
                /etc/foo/curr/baz.out
            ],
            base_dir=/base
        )
        [
            /base/bar.out,
            /base/curr/bar.out,
        ]
"""
rebase_and_copy_files = rule(
    implementation = _rebase_and_copy_files_impl,
    attrs = {
        "source_files": attr.label_list(
            mandatory = True,
            doc = "The list of source files to copy over. Ideally a list of filegroups",
        ),
        "base_dir": attr.string(
            mandatory = True,
            doc = "the directory to copy files into.",
        ),
        "prefix": attr.string(
            doc = "The keyword to cut at inclusively to rebase the files.",
        ),
    },
)

def files_to_dir(ctx, dirname, paths, post = ""):
    """Copies all the paths to a new directory tree removing symlinks.

    This macro is useful when you need to turn a list of depndency files,
    which bazel typically provides as symlinks, into a single directory
    output containing a copy of all those files.

    For example, you'd want to use this macro to package all files nicely
    in a directory tree without symlinks to allow the use of a 'mount --bind',
    a 'chroot', or the creation of a rootfs for kvm/qemu/uml.

    Args:
      dirname: string, the name to give to the directory.
      paths: list of File() objects representing the files to copy here.
      post: string, commands to run at the end of the copy. This is useful
          to perform post processing without having to create a separate
          script. The string is subject to {} .format() expansion, with
          keys "base" (bazel directory root of the build), "inputs" (list
          of input files as a shell array), and "dest" (destination
          directory).

    Returns:
      File() object representing the output directory.
    """
    d = ctx.actions.declare_directory(dirname)
    dest = shell.quote(d.path)

    roots = {}
    for f in paths:
      root = f.path
      if f.is_source and f.owner and f.owner.workspace_root:
          # The root of foreign source files is their workspace root.
          root = f.owner.workspace_root
      else:
          # Remove the ../ from short_path, if it exists.
          short_path = f.short_path
          if short_path.startswith('../'):
            short_path = short_path[3:]
          root = root[:-(len(short_path)+1)]

      if root not in roots:
          roots[root] = []

      if not root:
        roots[root].append(f.path)
        continue

      roots[root].append(f.path[len(root)+1:])

    pack = []
    for k, v in roots.items():
        pack.append("tar -C {root} -hc {files} |tar -x -C {dest}".format(
            root = shell.quote(k or "."),
            files = escape_and_join(v),
            dest = dest,
        ))

    base = d.path[:-len(d.short_path)]
    exps = dict(
        base = shell.quote(base),
        pack = "\n".join(list(reversed(pack))),
        dest = dest,
    )

    copy_command = """#!/bin/bash
set -euo pipefail
{pack}
{post}
""".format(
        post = post.format(**exps),
        **exps
    )
    script = ctx.actions.declare_file(dirname + "-copier.sh")
    ctx.actions.write(script, copy_command, is_executable = True)
    ctx.actions.run(
        outputs = [d],
        inputs = paths,
        executable = script,
        progress_message = "Writing files in a single directory %s..." % (dirname),
    )
    return d
