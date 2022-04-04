"""A set of rules to compare generated output against an expected data file."""

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
    script = """
        err=0
        act="{actual}"
        exp="{expected}"
        if [[ ! -e "${{act}}" ]]; then echo "Missing file: ${{act}}"; exit 1; fi
        if [[ ! -e "${{exp}}" ]]; then echo "Missing file: ${{exp}}"; exit 1; fi
        diff -u "${{exp}}" "${{act}}" ; RC=$?
        while [ "${{1:-}}" != "" ]; do
          if [[ ${{RC}} -ne 0 ]] && [[ "$1" == "--update_goldens" ]]; then
            b="$(readlink -f "${{exp}}")"
            echo "Updating ${{b}}"
            cp -vf "${{act}}" "${{b}}"
          fi
          shift
        done
        if [[ ${{RC}} != 0 ]]; then
          echo "Error: Generated file did not match ${{exp}}."
          echo ""
          echo "To automatically update your expected data files, run:"
          echo ""
          echo "  ./tools/update_goldens //path/to:your-diff_test  # a target"
          echo "  ./tools/update_goldens //path/to:all             # a dir"
          echo "  ./tools/update_goldens //path/to/...             # a tree"
          echo ""
        fi
        exit ${{RC}}
        """.format(
        actual = actual,
        expected = ctx.files.expected[0].short_path,
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
      identical.

      Typically, this would be used to compare the contents of a generated file
      against an expected data file.

      The easiest way to update the golden (expected) data files for your
      `diff_test` rules is to use the `update_goldens` utility in //tools.
      Some examples of use:

          ./tools/update_goldens //some/path/to:test  # one test
          ./tools/update_goldens //some/path/to:all   # all tests in a dir
          ./tools/update_goldens //some/path/to/...   # all tests in a tree

      Internally, this is what `update_goldens` is doing for each failing
      test:

          bazel run :some_diff_test -- --update_goldens

    """,
    implementation = _diff_test_impl,
    attrs = {
        "expected": attr.label(
            doc = "A label indicating the file containing the expected data.",
            allow_files = True,
        ),
        "actual": attr.label(
            doc = "A label indicating the file containing the actual data to check.",
            allow_files = True,
        ),
        "output_within_actual": attr.string(
            doc = "If actual is a target with multiple implicit outputs, the path to a specific output to test",
        ),
    },
    test = True,
)
