"""Functions to manipulate complex data types to assist in writing bzl rules."""

load("@bazel_skylib//lib:shell.bzl", "shell")

def escape_and_join(args):
    """Creates a shell-escaped string from a list of arguments."""
    escaped = []
    for x in args:
        if x.startswith("$"):
            # This is a hack because a rule in //fpga/defs.bzl wants to pass
            # $XCELIUM_PATH as an argument, and have the shell expand it correctly.
            escaped.append(x)
        else:
            escaped.append(shell.quote(x))
    cmd = " ".join(escaped)
    return cmd

def uniquify(iterable):
    """Uniquify the elements of an iterable."""
    elements = {element: None for element in iterable}
    return list(elements.keys())

def invert_label_keyed_string_dict(dictionary):
    """Returns a dictionary where keys are bazel label objects and values are strings.

    Bazel does not currently support a dictionary attr where keys are strings
    and values are a list of labels.

    Example:
    {"-y": [":abc", ":def"], "-x": [":abc"]} --> {":abc": "-x -y", ":def": "-y"}
    """
    result = dict()
    for key, val in dictionary.items():
        for label in val:
            if label in result:
                result[label] += " {}".format(key)
            else:
                result[label] = key
    return result

def expand_label_keyed_string_dict(target, args, short_path = False):
    """Returns a list of strings that contain arguments to file paths.

    This is helpful for tools that need to support generic command line
    arguments that reference multiple file targets in their bazel rule
    implementation. Each key could be a filegroup() that contains multiple
    bazel file objects.

    Example:
    {":abc": "-y", ":def": "-y"} --> ["-y", "abc", "-y", "def"]
    """
    result = []
    for f in target.files.to_list():
        for arg in args.split(" "):
            if short_path:
                result += [arg, f.short_path]
            else:
                result += [arg, f.path]
    return result
