load("@bazel_skylib//lib:paths.bzl", "paths")

""" Changes a path at a specified keyword to another path, appending the remaining onto it
Examples:
  >>> trim_path("/tree/foo/bar", "foo", "/etc")
  /etc/bar
"""

def rebase_path(path, prefix, base):
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
    all_input_files = [
        f
        for t in ctx.attr.source_files
        for f in t.files.to_list()
    ]
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

"""Copies all files in a directory to another directory, rebasing to a prefix as it goes along. If you wish to copy straight
into the directory, don't specify prefix.

Examples:
    >>> rebase_and_copy_files(
            source_files=[
                /etc/foo/bar.out,
                /etc/foo/curr/baz.out
            ],
            base_dir=/base,
            prefix=src
        )
    >>> [
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
    >>> [
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
