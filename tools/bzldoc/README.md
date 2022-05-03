# bzldoc

A simple documentation generator for bazel rules.

## Overview

bzldoc operates in three steps:

1. The `bzl2yaml` tool extracts metadata from a bazel .bzl file.

2. The `md.template` template is used to convert the metadata
   into markdown-formatted documentation.

3. The `mdfmt` tool reformats the generated markdown.

## Bazel integration

```
load(
    "//bazel/utils:diff_test.bzl",
    "diff_test",
)
load(
    "//tools/bzldoc:bzldoc.bzl",
    "bzldoc",
)

# Produces "gen-mymodule.md"
bzldoc(
    name = "gen-mymodule",
    src = "mymodule.bzl",
)

# This diff_test ensures that the copy of the expected
# file that is checked in stays in sync with what is
# generated.  The "--update_goldens" flag can be used
# to automatically regenerate the mymodule.md file.
diff_test(
    name = "mymodule.md-diff_test",
    expected = "mymodule.md",
    actual = "gen-mymodule.md",
)
```

