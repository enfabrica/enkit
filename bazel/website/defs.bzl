load("@bazel_skylib//lib:paths.bzl", "paths")
load("@bazel_skylib//lib:shell.bzl", "shell")
load("//bazel/utils:messaging.bzl", "fileowner", "location", "package")

WebsiteRoot = provider(
    doc = """A provider describing a web root.

A web root is a directory created with declare_directory() containing
a "www" subdirectory with all the files that need to be published on
a web server, with the paths mirroring the urls that need to be published. 

The directory ONLY contains files that need to be published, and no other
file. It is expected that a web server will make the entire "www" directory
available for access.
""",
    fields = {
        "root": "Path of the root of the website, a File object created with " +
                "declare_directory, containing a 'www' subdirectory",
    },
)

def _website_packaged_tree(ctx):
    shows = ctx.attr.show
    valid = ["copy", "source", "dest", "config"]
    for show in shows:
        if show not in valid:
            fail(location(ctx) + "invalid 'show' attribute: '{show}' - must be one of {valid}".format(show = show, valid = valid))

    root = ctx.actions.declare_directory(ctx.attr.name)
    webroot = paths.join(root.path, "www")

    configs = []
    inputs = []
    for target, labelstring in ctx.attr.labels.items():
        matches = [struct(**label) for label in json.decode(labelstring)]
        files = []
        for tfile in target.files.to_list():
            inputs.append(tfile)

            trel = tfile.short_path[len(tfile.owner.package) + 1:]
            troot = tfile.path[:-len(trel)]
            tpath = tfile.path
            if tfile.is_directory:
                trel = trel + "/"

            files.append(struct(trel = trel, troot = troot, tpath = tpath))
        configs.append(struct(package = package(target.label), files = files, matches = matches))

    config = ctx.actions.declare_file(ctx.attr.name + "-create.json")
    ctx.actions.write(config, content = json.encode_indent(configs, indent = "  "))

    args = ctx.actions.args()
    for arg in ["-p", package(ctx.label), "-w", webroot, "-c", config]:
        args.add(arg)
    for show in shows:
        args.add("-s")
        args.add(show)

    ctx.actions.run(
        outputs = [root],
        inputs = inputs + [config],
        executable = ctx.executable._copier,
        arguments = [args],
        tools = ctx.files._copier,
        mnemonic = "CopyingWebRoot",
    )
    return [DefaultInfo(files = depset([root])), WebsiteRoot(root = root)]

website_packaged_tree = rule(
    doc = """\
A rule to create a WebsiteRoot: a directory to be published on the web.

You should almost never use a website_packaged_tree directly. Instead,
use the macro website_tree to instantiate one.

The rule takes a set of labels, and a set of matches and parameters to
define which files - from which label - need to be copied where in the
output tree.

The output tree is expected to be published as is on a web site, so it
is important to:

   1) Exclude files that should not be copied.
   2) Place files in the correct directory - which may require moving
      files (or directories) from different targets into the same
      output directory.

This rule implements this behavior by:
    1) Creating a json file on disk defining the rules supplied.
    2) Invooking the //bazel/website/copier:copier tool to move the files.
""",
    implementation = _website_packaged_tree,
    attrs = {
        "labels": attr.label_keyed_string_dict(
            allow_files = True,
            mandatory = True,
            doc = """\
Dictionary where each key is a label whose files/dirs should be copied in
the WebsiteRoot, while the value is a json encoded string representing an
array of structs created with the 'website_paths' macro:
        struct(match = match, dest = dest, strip = strip)
and representing the set of files to copy from the input targets.
""",
        ),
        "show": attr.string_list(
            doc = "Debug info to show, one or more of 'copy, config, source, dest'",
        ),
        "_copier": attr.label(
            doc = "Path to the tool used to copy the files in place",
            default = Label("//bazel/website/copier:copier"),
            executable = True,
            cfg = "host",
        ),
    },
)

def website_tree(name, paths, **kwargs):
    """"""
    labelpaths = {}
    for path in paths:
        if not path or not hasattr(path, "targets") or not hasattr(path, "paths"):
            fail("website_tree rule '{name}' in '{package}' only accepts dependencies created with website_path - got {got}".format(name = name, package = native.package_name(), got = path))

        for target in path.targets:
            config = labelpaths.setdefault(target, [])
            config.extend(path.paths)

    labelconfig = {}
    for label, config in labelpaths.items():
        labelconfig[label] = json.encode(config)

    website_packaged_tree(name = name, labels = labelconfig, **kwargs)

def website_path(targets, path = None, paths = None, strip = []):
    if (not path and not paths) or (path and paths):
        fail("website_path for '{target}' in '{package}' allows exactly one of 'path' or 'paths' attributes".format(target = targets, package = native.package_name()))

    if not paths:
        paths = {"*": path}

    confs = []
    for match, dest in paths.items():
        confs.append(struct(match = match, dest = dest, strip = strip))
    return struct(targets = targets, paths = confs)
