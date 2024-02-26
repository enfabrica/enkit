"""Rules and macros to deal with appengine configuration files."""

load("@bazel_skylib//lib:shell.bzl", "shell")
load("//bazel/website:defs.bzl", "WebsiteRoot")
load("//bazel/utils:files.bzl", "write_to_file")

def appengine_config(name, config, **kwargs):
    """Creates an app.yaml config file from the config provided.

    Args:
      config: dict or struct, representing the config to be serialized
          in the configuration file.
      **kwargs: additional parameters passed on as is to the 'write_to_file' rule.
          Most useful to add tags or visibility rules.

    Notes:
      app.yaml is, well, in yaml format. This function cheats by serializing
      the config in json format in the yaml file. Given that json is mostly
      a subset of yaml, this works well in practice.
    """
    if not type(config) == "str":
        config = json.encode(config)

    write_to_file(name = name, output = name + "/app.yaml", content = config, **kwargs)

def _appengine_config_merge(ctx):
    args = []
    if ctx.attr.login:
        args.append("-x")
        args.append(shell.quote("login: " + ctx.attr.login))
    if ctx.attr.secure:
        args.append("-x")
        args.append(shell.quote("secure: " + ctx.attr.secure))
    for extra in ctx.attr.extra:
        args.append("-x")
        args.append(shell.quote(ctx.attr.extra))

    for hdr in ctx.attr.headers:
        args.append("-e")
        args.append(shell.quote(hdr))

    inputs = []
    for tomerge in ctx.files.before + ctx.files.merge + ctx.files.after:
        args.append(shell.quote(tomerge.path))
        inputs.append(tomerge)

    outfile = ctx.actions.declare_file(ctx.attr.name + "/app.yaml")
    ctx.actions.run_shell(
        outputs = [outfile],
        inputs = inputs,
        tools = ctx.files._merger,
        command = "{merger} {args} > {outfile}".format(
            merger = ctx.executable._merger.path,
            args = " ".join(args),
            outfile = outfile.path,
        ),
    )
    return DefaultInfo(files = depset([outfile]))

# Basic content of an app.yaml file for a python app.
DEFAULT_BASE = struct(
    runtime = "python312",
)

# Basic handlers typically defined in an app.yaml file.
#
# Serves the entire content of the www/ subdirectory.
DEFAULT_HANDLERS = struct(
    handlers = [
        struct(
            url = "/",
            static_dir = "www/",
        ),
    ],
)

# Snippets of config to prepend to any merged app.yaml file.
DEFAULT_CONFIG_BEFORE = [
    Label("//bazel/appengine:config-base"),
]

# Snippets of config to append to any merged app.yaml file.
DEFAULT_CONFIG_AFTER = [
    Label("//bazel/appengine:config-handlers"),
]

DEFAULT_HEADERS = [
    # See https://cloud.google.com/appengine/docs/standard/python/how-requests-are-handled#caching_static_content
    # for why this is necessary.
    #
    # Tl;Dr: without it, proxies will not use the Accept-Encoding header as part
    # of the key of the cache, so may return gzipped content for browsers that
    # don't accept it and vice-versa.
    "Vary: Accept-Encoding",
]

appengine_config_merge = rule(
    doc = """\
Merges multiple snippets of app.yaml files, with some normalization.

For example, let's say you have rules like:

    appengine_config(
        name = "api-version",
        config = {
            "api_version": 5,
        },
    )
    
    appengine_config(
      name = "api-runtime",
      config = {
          "runtime": "go"
      },
    )

A rule like:

    appengine_config_merge(
        name = "config",
        before = [],
        merge = [":api-version", ":api-runtime"],
        after = [],
    )

will generate a config file that looks like:

    api_version: 5
    runtime: "go"

Note that the before and after parameters above are important.
appengine_config_merge is opinionated: by default, it will try
to create a valid app.yaml file for python27, with the best
options set to require authentication, https, and caching.

You can use attributes of the rule to tune the behaviore and
relax those settings if necessary.

Most of the attributes in the rule map to properties of the
app.yaml file format. Read:

  https://cloud.google.com/appengine/docs/standard/python/config/appref

for more details.
""",
    implementation = _appengine_config_merge,
    attrs = {
        "login": attr.string(
            doc = "Sets the 'login' property in every handler defined.",
            default = "required",
        ),
        "secure": attr.string(
            doc = "Sets the 'secure' property in every handler defined.",
            default = "always",
        ),
        "extra": attr.string_list(
            doc = """\
Sets arbitrary extra properties in each handler defined.
Each string in the list must be a key value string like
"Foo: bar".""",
            default = [],
        ),
        "headers": attr.string_list(
            doc = """\
Sets arbitrary headers returned by handlers defined.
Each string in the list must be a valid http header,
like "Content-Type: text/html".""",
            default = DEFAULT_HEADERS,
        ),
        "before": attr.label_list(
            doc = """\
Set of app.yaml snippets to merge first in the output file.
Order is important: following snippets override the parameters
of previous snippets.""",
            allow_files = [".yaml", ".yml"],
            default = DEFAULT_CONFIG_BEFORE,
        ),
        "merge": attr.label_list(
            doc = """\
Set of app.yaml snippets to merge in the output file.
Order is important: following snippets override the parameters
of previous snippets.""",
            allow_files = [".yaml", ".yml"],
            mandatory = True,
        ),
        "after": attr.label_list(
            doc = """\
Set of app.yaml snippets to merge last in the output file.
Order is important: following snippets override the parameters
of previous snippets.""",
            allow_files = [".yaml", ".yml"],
            default = DEFAULT_CONFIG_AFTER,
        ),
        "_merger": attr.label(
            default = Label("//bazel/appengine/configtools:merge"),
            executable = True,
            cfg = "host",
        ),
    },
)

def _appengine_config_index(ctx):
    args = []
    for index in ctx.attr.index:
        args.append("-i")
        args.append(index)

    inputs = []
    if ctx.attr.template:
        args.append("-t")
        args.append(ctx.attr.template.path)
        inputs.append(ctx.attr.template)

    root = ctx.attr.root[WebsiteRoot].root.path
    args.append("-r")
    args.append(shell.quote(root))

    args.append("-u")
    args.append(shell.quote(root + "/www"))
    args.append(shell.quote(root + "/www"))

    outfile = ctx.actions.declare_file(ctx.attr.name + "/app.yaml")
    ctx.actions.run_shell(
        outputs = [outfile],
        inputs = inputs + ctx.files.root,
        tools = ctx.files._indexer,
        command = "{indexer} {args} > {outfile}".format(
            indexer = ctx.executable._indexer.path,
            args = " ".join(args),
            outfile = outfile.path,
        ),
    )
    return DefaultInfo(files = depset([outfile]))

appengine_config_index = rule(
    doc = """\
Generates a snippet of app.yaml file with redirects for index files.

AppEngine is capable of directly serving static trees of files.

However, it does not have a feature allowing to map a directory
name to an index file. For example, to configure www.whatever.com/foo/
to transparently serve www.whatever.com/foo/index.html.

This rule scans a directory tree looking for pre-defined index
files. If found, it creates snippets of app.yaml file configuring
AppEngine explicitly to load those index files when queried for
the corresponding directory.

For example, given a directory hierarchy containing:

  www/top/index.html

It will generate a handler containing a snippet similar to:

  - url: /top/
    static_files: /top/index.html
    upload: /top/index.html
""",
    implementation = _appengine_config_index,
    attrs = {
        "root": attr.label(
            doc = "Target defining the root of the website, a WebsiteRoot",
            mandatory = True,
            providers = [WebsiteRoot],
        ),
        "template": attr.label(
            doc = """\
A template of the handler to output for each index.

The template can contain variables {filename}, {filedir}, {urldir},
{urlfile}, {mimetype}, {mimeencoding}.
""",
            allow_single_file = True,
        ),
        "index": attr.string_list(
            doc = "Set of files to consider as index, will be searched in order.",
            default = ["index.html", "index.htm"],
        ),
        "_indexer": attr.label(
            default = Label("//bazel/appengine/configtools:index"),
            executable = True,
            cfg = "host",
        ),
    },
)
