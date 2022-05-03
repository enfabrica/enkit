# `codegen.bzl` Documentation

<!-- TODO(jonathan): make this template better. -->

## Overview

Test bzl file for bzldoc.

This is a phoney baloney bzl file for testing bzldoc.

It is a fork of codegen.bzl.

Usage:

```
load("//tools/bzldoc/testdata:codegen.bzl", <rule names...>)
```

Source code:
https://github.com/enfabrica/internal/tools/bzldoc/testdata/codegen.bzl

## Rules

### `codegen` Rule

Runs codegen to combine templates and data files to an artifact.

<table><thead><tr>
<th> Attribute </th><th> Type </th><th> Description </th>
</tr></thead><tbody>
<tr><td>

data

</td><td>

[label_list](https://docs.bazel.build/versions/main/skylark/lib/attr.html#label_list)

</td><td>

An ordered list of data files to load.

</td></tr>
<tr><td>

outs

</td><td>

[output_list](https://docs.bazel.build/versions/main/skylark/lib/attr.html#output_list)

</td><td>

Artifacts to generate.

</td></tr>
<tr><td>

srcs

</td><td>

[label_list](https://docs.bazel.build/versions/main/skylark/lib/attr.html#label_list)

</td><td>

A list of jinja2 template files to import.

</td></tr>
<tr><td>

schema

</td><td>

[label](https://docs.bazel.build/versions/main/skylark/lib/attr.html#label)

</td><td>

A jsonschema file to check the imported data against.

</td></tr>
<tr><td>

overrides

</td><td>

[string_list](https://docs.bazel.build/versions/main/skylark/lib/attr.html#string_list)

</td><td>

A pair of key=value pairs to override context data.

</td></tr>
<tr><td>

template_name

</td><td>

[string](https://docs.bazel.build/versions/main/skylark/lib/attr.html#string)

</td><td>

The specific jinja2 template to render (optional).

</td></tr>
<tr><td>

multigen_mode

</td><td>

[bool](https://docs.bazel.build/versions/main/skylark/lib/attr.html#bool)

</td><td>

Enable multigen mode.

</td></tr>
<tr><td>

codegen_tool

</td><td>

[label](https://docs.bazel.build/versions/main/skylark/lib/attr.html#label)

</td><td>

The path to the codegen tool itself.

</td></tr>
</tbody></table>

## Macros

### `codegen_test` Macro

Usage: `codegen_test(name, expected)`

Missing documentation.

______________________________________________________________________

_Documentation generated with bzldoc._
