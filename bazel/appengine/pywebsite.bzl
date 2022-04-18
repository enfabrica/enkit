"""Set of rules and macros to start and push an appengine website"""

load("//bazel/website:defs.bzl", "WebsiteRoot")
load("//bazel/appengine:config.bzl", "appengine_config_index", "appengine_config_merge")
load("//bazel/utils:messaging.bzl", "package")
load("//bazel/utils:macro.bzl", "mconfig", "mcreate_rule")

APPENGINE_RUNNER_ATTRS = {
    "root": attr.label(
        providers = [WebsiteRoot],
        mandatory = True,
        doc = "The root of the website to run the command in",
    ),
    "config": attr.label(
        allow_single_file = True,
        mandatory = True,
        doc = "The app.yaml configuration file to use with the gcloud commands",
    ),
}

def appengine_runner(ctx, name, prepare, execute, **kwargs):
    """Executes the specified command in a directory suitable for gcloud.

    Args:
      ctx: a rule context.
      name: name of the script to generate. It is appended to ctx.attr.name
        by this function.
      prepare: string, shell commands to add to finish the setup of the
        directory. This command should create the desired app.yaml file in
        the location defined by the $cleaned environment variable.
      execute: string, shell command to run the desired appengine command.
       This command is run in a directory containing exactly the app.yaml
       file stored in $cleaned, and a www/ dir with the files to push.

    Returns:
      An executable provider and DefaultInfo, suitable to be directly
      returned by the rule.
    """
    template = """#!/bin/bash
set -e

tempdir="$(mktemp -d run.XXXXXX)"
cleaned="$tempdir/app.yaml"

{prepare}
ln -sf "$(realpath {webroot})/www" "$tempdir/www"

cd "$tempdir"
echo ================= SERVING FILES ======================
find -L .
echo ======================================================
echo = running target: {package}
echo = root: {binroot}
echo = original config: {configpath}
echo = cleaned config: $(realpath app.yaml)
echo = served files: $(realpath www)
echo ======================================================
{execute}
echo ========== DONE - server terminated ==================
"""
    root = ctx.attr.root[WebsiteRoot]

    webroot = root.root.short_path
    configpath = ctx.file.config.short_path
    configname = ctx.file.config.basename

    subs = dict(
        configpath = configpath,
        configname = configname,
        package = package(ctx.label),
        binroot = ctx.bin_dir.path,
        webroot = webroot,
        **kwargs
    )

    output = ctx.actions.declare_file(ctx.attr.name + "/" + name)

    prepare = prepare.format(**subs)
    execute = execute.format(**subs)
    expanded = template.format(prepare = prepare, execute = execute, **subs)

    ctx.actions.write(output, expanded, is_executable = True)
    return [DefaultInfo(
        executable = output,
        runfiles = ctx.runfiles(
            files = [ctx.file.config],
            transitive_files = depset([root.root]),
        ),
    )]

def _appengine_website_run(ctx):
    prepare = """egrep -v "app_engine_apis:|login:" {configpath} > "$cleaned";"""
    execute = """\
echo -e '=    open \\e]8;;http://127.0.0.1:8080/\\ahttp://127.0.0.1:8080/\\e]8;;\\a to see the pages    ='
echo ======================================================
appserver=$(which dev_appserver.py || true)
appserver=${{appserver:-/usr/lib/google-cloud-sdk/bin/dev_appserver.py}}

"$appserver" --runtime {runtime} "$@" "app.yaml"
"""
    return appengine_runner(
        ctx,
        "runner.sh",
        prepare,
        execute,
        runtime = ctx.attr.runtime,
    )

appengine_website_run = rule(
    doc = """\
Starts a web server on localhost serving the specified app.yaml and root""",
    implementation = _appengine_website_run,
    executable = True,
    attrs = dict({
        "runtime": attr.string(
            default = "python27",
            doc = "The AppEngine runtime to use",
        ),
    }, **APPENGINE_RUNNER_ATTRS),
)

def _appengine_website_push(ctx):
    prepare = """ln -sf "$(realpath {configpath})" "$cleaned";"""
    execute = """{gcloud} app deploy --quiet --project={project} "$@";"""
    return appengine_runner(
        ctx,
        "pusher.sh",
        prepare,
        execute,
        gcloud = ctx.attr.gcloud,
        project = ctx.attr.project,
    )

appengine_website_push = rule(
    doc = "Pushes a web server to the specified gcloud project",
    implementation = _appengine_website_push,
    executable = True,
    attrs = dict({
        "project": attr.string(
            mandatory = True,
            doc = "AppEngine project to push to",
        ),
        "gcloud": attr.string(
            mandatory = False,
            default = "/usr/bin/gcloud",
            doc = "Path to the system installed gcloud binary",
        ),
    }, **APPENGINE_RUNNER_ATTRS),
)

def appengine_website(name, root, index = {}, config = {}, push = {}, run = {}, **kwargs):
    """Creates targets to manage an appengine based website.

    Given a name "name", this macro will create the targets:
      {name}-index - an appengine_config_index target, to scan the file in the
          tree and preapre app.yaml hanlders.
      {name}-config - an appengine_config_merge target, to create fill the
          other necessary app.yaml configuration parameters.
      {name}-push - an appengine_website_push target, to run the commands to
          push the website to appengine.
      {name} - an appengine_website_run target, to run a local webserver
          with the defined website.

    Args:
      root: label, the root of the WebSite, most likely a website_tree target.
      index, config, push, run: objects created with mconfig(), defining the parameters
        for the {name}-index, {name}-config, {name}-push, {name} instantiated targets.
    """
    index = mcreate_rule(
        name,
        appengine_config_index,
        "index",
        index,
        kwargs,
        mconfig(root = root),
    )
    config = mcreate_rule(
        name,
        appengine_config_merge,
        "config",
        config,
        kwargs,
        mconfig(merge = [index]),
    )
    push = mcreate_rule(
        name,
        appengine_website_push,
        "push",
        push,
        kwargs,
        mconfig(config = config, root = root),
    )
    run = mcreate_rule(
        name,
        appengine_website_run,
        "",
        run,
        kwargs,
        mconfig(config = config, root = root),
    )
