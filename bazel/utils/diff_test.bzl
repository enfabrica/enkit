"""A set of rules to compare generated output against an expected data file.

Quick Start:

1. For a given rule ":foo" that generates a "foo.txt" file, create
a diff_test rule:

    diff_test(
        name = "foo-diff_test",
        actual = ":foo",
        expected = "expected/foo.txt",
    )

2. Use "touch" to create an empty expected data file:

    touch expected/foo.txt

3. Run the diff_test rule to update the empty expected data file:

    bazel run :foo-diff_test -- --update_goldens

4. Manually inspect the updated `expected/foo.txt` file to ensure
   that is correct.  From now on, the `:foo-diff_test` rule will
   fail if the generated `foo.txt` ever changes in a way that does
   not match the expected `expected/foo.txt`
"""

def _zipdiff_test_impl(ctx):
    # Note: in python format strings, {{ and }} render as { and }.
    script = """
        err=0
        act="{actual}"
        exp="{expected}"
        if [[ ! -e "${{act}}" ]]; then echo "Missing file: ${{act}}"; exit 1; fi
        if [[ ! -e "${{exp}}" ]]; then echo "Missing file: ${{exp}}"; exit 1; fi
        actdir="$(mktemp -d -p ${{TEST_TMPDIR}})"
        unzip "${{act}}" -d "${{actdir}}"
        expdir="$(mktemp -d -p ${{TEST_TMPDIR}})"
        unzip "${{exp}}" -d "${{expdir}}"
        diff -r -u "${{actdir}}" "${{expdir}}"; RC=$?
        while [ "${{1:-}}" != "" ]; do
          if [[ ${{RC}} -ne 0 ]] && [[ "$1" == "--update_goldens" ]]; then
            b="$(readlink -f "${{exp}}")"
            echo "Updating ${{b}}"
            cp -vf "${{act}}" "${{b}}"
            RC=0  # clear the error if we updated goldens
          fi
          shift
        done
        exit ${{RC}}
        """.format(
        actual = ctx.files.actual[0].short_path,
        expected = ctx.files.expected[0].short_path,
    )
    ctx.actions.write(
        output = ctx.outputs.executable,
        content = script,
    )
    runfiles = ctx.runfiles(files = ctx.files.actual + ctx.files.expected)
    return [DefaultInfo(runfiles = runfiles)]

zipdiff_test = rule(
    doc = """
      A test that compares the contents of two zip files to ensure the contents
      are identical.

      Typically, this would be used to compare the contents of a generated file
      against an expected data file.

      A quick way to update expected data files:

          blaze run :some_diff_test -- --update_goldens

    """,
    implementation = _zipdiff_test_impl,
    attrs = {
        "expected": attr.label(
            doc = "A label indicating the file containing the expected data.",
            allow_files = True,
        ),
        "actual": attr.label(
            doc = "A label indicating the file containing the actual data to check.",
            allow_files = True,
        ),
    },
    test = True,
)

def _diff_test_impl(ctx):
    if ctx.attr.output_within_actual:
        actual = ctx.attr.output_within_actual
    else:
        if len(ctx.files.actual) > 1:
            fail("`output_within_actual` must be specified when `actual` target has multiple outputs")
        actual = ctx.files.actual[0].short_path

    # Note: in python format strings, {{ and }} render as { and }.
    diff_tool = "diff -u"
    if ctx.attr.binary:
        diff_tool = "cmp"
    script = """
        err=0
        act="{actual}"
        exp="{expected}"
        if [[ ! -e "${{act}}" ]]; then echo "Missing file: ${{act}}"; exit 1; fi
        if [[ ! -e "${{exp}}" ]]; then echo "Missing file: ${{exp}}"; exit 1; fi
        {diff_tool} "${{exp}}" "${{act}}" ; RC=$?
        while [ "${{1:-}}" != "" ]; do
          if [[ "$1" == "--update_goldens" ]]; then
            echo "--update_goldens specified."
            if [[ ${{RC}} -ne 0 ]]; then
              b="$(readlink -f "${{exp}}")"
              echo "Updating ${{b}} from ${{act}}"
              cp -vf "${{act}}" "${{b}}"
              RC=0
            else
              echo "${{act}} is already up-to-date."
            fi
          fi
          shift
        done
        if [[ ${{RC}} != 0 ]]; then
          echo "Error: Generated file did not match ${{exp}}."
          echo ""
          echo "To automatically update your expected data file, run:"
          echo ""
          echo "  bazel run ${{TEST_TARGET}} -- --update_goldens"
          echo ""
          echo "Or use the \"update_goldens\" script to update many expected"
          echo "data files at once."
          echo ""
        fi
        exit ${{RC}}
        """.format(
        actual = actual,
        expected = ctx.files.expected[0].short_path,
        diff_tool = diff_tool,
    )
    ctx.actions.write(
        output = ctx.outputs.executable,
        content = script,
    )
    runfiles = ctx.runfiles(files = ctx.files.actual + ctx.files.expected)
    return [DefaultInfo(runfiles = runfiles)]

diff_test = rule(
    doc = """
      A test that compares the contents of two files to ensure they are
      identical.  Typically, this would be used to compare the output of a
      file-generating rule against an expected data file.

      To update the expected data file to match that actual data file, run your
      test (using `bazel run`, not `bazel test`) with the `--update_goldens`
      flag specified.  For example:

          bazel run //path/to:your-diff_test -- --update_goldens

      Alternately, consider using the provided `update_goldens` python script
      as a quick way to identify and regenerate a large number of expected data
      files at once.

      If your generating rule produces multiple files, consider using the
      provided `extract_file` rule to winnow your depset down to a single
      file.
    """,
    implementation = _diff_test_impl,
    attrs = {
        "expected": attr.label(
            doc = "A label indicating the file containing the expected data.",
            allow_files = True,
            mandatory = True,
        ),
        "actual": attr.label(
            doc = "A label indicating the file containing the actual data to check.",
            allow_files = True,
        ),
        "output_within_actual": attr.string(
            doc = "If actual is a target with multiple implicit outputs, the path to a specific output to test",
        ),
        "binary": attr.bool(
            doc = "Enable to use `cmp` instead of `diff` for this file.",
            default = False,
        ),
    },
    test = True,
)

def diff_test_suite(name, files, subdir = "expected", **kwargs):
    """diff_test_suite is a macro for quickly instantiating multiple diff_test rules.

    A common design pattern in our BUILD files is to use a list comprehension to implement
    many diff_test targets.  This macro replaces those diff_tests, forces the placement
    of the expected data files to be consistent, and implements a consistent test naming.

    All generated diff_test rules will be named "<filename>-diff_test".

    "diff_test_suite" should not be confused with "multi_diff_test" which is a specialized
    rule for implementing a difference test against the complete set of all files produced
    by one or more bazel targets.

    Arguments:
      name: name of the generated test suite.
      files: a list of generated file targets to test.
      subdir: a subdirectory beneath the current directory where expected data files are
        populated.  This must be a subdirectory beneath the directory containing the
        current BUILD file, and that directory must not contain a BUILD file of its
        own.  Defaults to "expected".
      kwargs: additional keyword arguments to pass to all diff_tests.
    """
    [
        diff_test(
            name = "%s-diff_test" % f,
            expected = "%s/%s" % (subdir, f),
            actual = f,
            **kwargs
        )
        for f in files
    ]
    native.test_suite(
        name = name,
        tests = ["%s-diff_test" % f for f in files],
    )

def _extract_file_impl(ctx):
    selected_file = None
    for f in ctx.files.target:
        if f.path == ctx.attr.path or f.short_path == ctx.attr.path:
            selected_file = f
    if selected_file == None:
        fail("%s: %s not found in %r" % (ctx.attr.name, ctx.attr.path, [x.short_path for x in ctx.files.target]))
    return [DefaultInfo(files = depset([selected_file]))]

extract_file = rule(
    doc = """
      Filters the depset provided by a target down to a single file, indicated by a path.

      This rule will fail if a matching file is not found.
    """,
    implementation = _extract_file_impl,
    attrs = {
        "target": attr.label(
            doc = "A label indicating a depset of one or more files.",
            mandatory = True,
        ),
        "path": attr.string(
            doc = "The path to the file to extract from the depset.",
            mandatory = True,
        ),
    }
)
