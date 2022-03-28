"""A set of rules to compare generated output against an expected data file.

Example of use:

  genrule(
     name = "foobar.txt-gen",


"""

def _diff_test_impl(ctx):
    if len(ctx.files.actual) > 1:
        fail("`actual` must specify a single file.")
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
            mandatory = True,
        ),
    },
    test = True,
)

