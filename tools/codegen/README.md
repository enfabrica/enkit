# codegen

A simple jinja2-driven code generator.

## Overview

The `codegen` tool combines a number of data files (in json or yaml
format) with a jinja2 template to produce a code artefact.  Bazel
rules are also provided to integrate this automated code production
into a bazel build.

## Bazel Usage

### `codegen`

Codegen can be invoked via a bazel BUILD rule as shown:

```
load("//tools/codegen:codegen.bzl", "codegen")

codegen(
    name = "foo_bar.sv-gen",
    srcs = ["foo.sv.jinja2'],  # the template
    data = ["bar.yaml"],  # the data
    out = "foo_bar.sv"  # the bar version of the foo template.
)
```

`codegen` outputs can then be used directly as sources for
other rules:

```
verilog_library(
    name = "foo_bar",
    srcs = ["foo_bar.sv"],
)
```

In normal mode, codegen combines data with a template file in order to produce
a single output file.  "multigen mode" is available for flows where multiple
files are produced from a single configuration file.  For example:

```
load("//tools/codegen:codegen.bzl", "codegen")

codegen(
    name = "multimode_test-zip",
    out = "multimode_test.zip",
    srcs = ["multimode_test.jinja2"],
    data = ["multimode.yaml"],
    multigen_mode = 1,
)
```

### multigen mode

In "multigen" mode, codegen expects to find a data context comprising a
dictionary.  Each key of the dictionary is the name of the output file to
generate, and the value associated with that key is the data context to present
to the template to produce that file.

If the dictionary contains the special key `_default`, then each file's data
context is formed by merging the file's data structure onto the `_default` data
structure, using the rules defined in Merging Data, below.

Bazel doesn't play well with tools that generate a set of files that aren't
pre-defined in the `BUILD.bazel` file.  "multigen mode" works around this
issue by generating it's arbitrary set of output files as a single zip file.
The "out" attribute of the codegen rule must specify the name of a zip file
to write to.

## Inputs

`codegen` supports YAML and JSON data files.

### Merging Data

Data files are loaded in the order they are specified on the command line.
Subsequent data is merged with prior data according the the following rules:

   * Scalar values override previous scalar values.
   * Lists are appended onto previous lists.
   * Dictionaries are merged by inserting all new keys into the existing
     dictionary.
   * When both old and new dictionaries define the same key, the new and old
     data values are merged (recursively) using the rules defined here.

The YAML parser has been slightly modified to combine data from multiply defined
keys even within the same YAML file.  Thus:

```
foo:
  - bar

foo:
  - bum
```

is equivalent to:

```
foo:
  - bar
  - bum
```

### Data Validation

TODO.

## Template language

`codegen` implements the `Jinja2` template engine.  `Jinja2` is fully
documented here:

* [Jinja2 Template Designer Documentation](https://jinja.palletsprojects.com/en/3.1.x/templates/)
* [Jinja2 Library Documentation](https://jinja.palletsprojects.com/en/3.1.x/)

### Jinja2 in 60 seconds

Jinja2 has 3 main language constructs:

```
{% statements %}
{{ expressions }}
{# comments #}
```

Statements are things like conditional expressions, for loops, and variable assignment.  For example:

```
{% set ns = namespace() %}
{% for item in biglist %}
{%   if "attribute" in item %}
{%      set ns.foo = item.attribute %}
{%   endif %}
{% endfor %}
```

Expressions evaluate to strings that are inserted into the rendered output.  Expressions
can include pipelines of Jinja2 filters.  For example:

```
// {{ ns.foo | wordwrap(75, wrapstring="\n// ") }}
```

Comments are omitted from rendered output.

### Removing excess whitespace

It is often desirable to remove extra whitespace on either side of a construct
(a statement, expression, or comment).  The "-" character just inside a
construct block tells Jinja2 to remove all whitespace on that side of the
construct.

For example:

```
Removes all whitespace         {{- " to the left" }} of a construct.
Removes all whitespace {{ "to the right " -}}        of a construct.
Removes all whitespace
{{- " on either side " -}}
      of a construct.
```

Renders as:

```
Removes all whitespace to the left of a construct.
Removes all whitespace to the right of a construct.
Removes all whitespace on either side of a construct.
```

Whitespace gobbling is interrupted by comments.  For example:

```
{%- set foo = 1 + 1 %}
{# intentional blank line #}
{%- set bar = 2 * 2 %}
```

In the above example, the newline at the end of the 2nd line is removed by the
`{%-` construct on the third line.  However, the newline at the end of the 1st
line is preserved (whitespace gobbling ends at the comment).

### Template extensions

The following generic jinja2 extensions are enabled within codegen:

* [Expression
  Statements](https://jinja.palletsprojects.com/en/3.1.x/extensions/#expression-statement):
  Adds the "do" statement, to evaluate expressions within statements.
* [Loop
  Controls](https://jinja.palletsprojects.com/en/3.1.x/extensions/#debug-extension):
  Adds "break" and "continue" in loops.
* [debug](https://jinja.palletsprojects.com/en/3.1.x/extensions/#debug-extension):
  Adds the "debug" statement to dump debugging information from within the
  template.

Additionally, codegen adds the following functions:

* `bitwise_and(x, y)`: performs a bitwise and operation for two integers.
* `bitwise_or(x, y)`: performs a bitwise or operation for two integers.
* `bitwise_xor(x, y)`: performs a bitwise xor operation for two integers.
* `bitwise_not(x)`: performs a bitwise not operation for an integer.
