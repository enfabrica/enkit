"""Helpers to facilitate the creation of macros that instantiate rules.

When writing a macro that instantiates multiple rules, one common problem
is how to forward additional kwargs to each rule instantiated.
For example, how to correctly forward "//visibility", or how to forward
"tags" or exec environment.

Additionally, when defining macros that create deploy rules as well as
test rules, it's common to have some "base parameters" that are shared
in a BUILD file, together with some parameters that are customized per
rule.

The functions and data types in this file help provide a generic
framework to handle those cases.
"""

load("//bazel/utils:merge_kwargs.bzl", "merge_kwargs")

def mconfig(*args, **kwargs):
    """Creates or overrides a dict representing rule configs.

    This macro is normally used in BUILD.bazel files to define the
    attributes of a rule or another macro.

    This macro takes a list of dictionaries (*args) and key value
    pairs (as **kwargs) and overlay them on top of each other, using
    the semantics defined for merge_kwargs (scalars replace, dicts are
    merged, lists are appended - uniquely).

    For example:
      >>> d1 = mconfig(foo = 1, bar = [2], baz = {'a': 1})
      {"foo": 1, "bar": [2], "baz": {'a': 1}}
      >>> mconfig(d1, foo = 2, bar = [3], baz = {'b': 2})
      {"foo": 2, "bar": [2, 3], "baz": {'a': 1, 'b': 2}}
    """
    args = list(args) + [kwargs]
    if len(args) <= 1:
        return args[0]

    result = args[0]
    for arg in args[1:]:
        result = merge_kwargs(result, arg)
    return result

def mconfig_get(defs, *args, default = {}):
    """Returns the value of the key supplied as *args in a config object.

    This macro is normally used from within other macros defined in
    *.bzl files, to retrieve the value from a dict, recursively.

    Args:
      *args: keys, used one after another.
      default: default value to return if the key is not found.

    Example:
      >>> d1 = mconfig(srcs = [...], astore = mconfig(url = "uuu"), f = {'a': 1})
      {"srcs": [...], "astore": {"url": "uuu"}, f: {'a': 1}}
      >>> mconfig_get(d1, "astore", "url")
      "uuu"
      >>> mconfig_get(d1, "f", "a")
      1
      >>> mconfig_get(d1, "invalid")
      {}

    Returns:
      For example, if *args is ("config", "astore", "root"), the code will
      return the equivalent of defs["config"]["astore"]["root"].
    """
    if defs == None:
        return default

    for a in args[:-1]:
        defs = defs.get(a, {})
    return defs.get(args[-1], default)

def mcreate_rule(current_name, rule_type, suffix, arg, *args):
    if type(arg) == "string":
        return arg

    rargs = {}
    for cfg in list(args) + [arg]:
        rargs = merge_kwargs(rargs, cfg)

    name = current_name
    if suffix:
        name = name + "-" + suffix
    rule_type(
        name = name,
        **rargs
    )
    return ":" + name
