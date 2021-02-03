ContainerTestProvider = provider(fields=["before", "after", "out"])

# emulation docker will run a ContainerTestProvider for [n_run_test] to run. I specifies a spin up script, a close script,
# and an artifact file that the close script will use.
def _emulation_docker(ctx):
    print("should be emulating docker here")
    env_file = ctx.actions.declare_file("hello_env.sh")
    ctx.actions.expand_template(
        is_executable=True,
        output=ctx.outputs.run_container,
        substitutions= {
            "example":"the cat says meow",
            "ENVIRONMENT_FILE": env_file.short_path,
        },
        template=ctx.file._open_template
    )
    print("post template")
    ctx.actions.run(
        executable=ctx.outputs.run_container,
        outputs=[env_file]
    )
    print("pre expand close template")
    ctx.actions.expand_template(
        is_executable=True,
        output=ctx.outputs.close_container,
        substitutions= {
            "example":"the cat says meow",
            "ENVIRONMENT_FILE": "hello",
        },
        template=ctx.file._close_template
    )
    print("post expand close template")

    return [
        DefaultInfo(
            runfiles=ctx.runfiles([ctx.outputs.run_container, ctx.outputs.close_container]),
            files=depset([ctx.outputs.run_container, ctx.outputs.close_container, env_file])
        ),
        ContainerTestProvider(
            before=ctx.outputs.run_container,
            out=env_file,
            after=ctx.outputs.close_container
        )
    ]


run_container = rule(
    _emulation_docker,
    attrs = {
        "image_name" : attr.string(),
        "port_bind": attr.string_list(),
        "env_prefix": attr.string(),
         "_open_template": attr.label(
                default = Label("//bazel/testing:run_container.sh"),
                allow_single_file = True,
        ),
        "_close_template": attr.label(
                default = Label("//bazel/testing:close_container.sh"),
                allow_single_file = True,
        )
    },
    outputs = {
        "close_container": "%{name}_close.sh",
        "run_container": "%{name}_run.sh",
    },
)

def _run_test(ctx):
    print("here")
    files_to_run_after=[]
    raw_executable_paths = []

    for container in ctx.attr.containers:
        container_provider = container[ContainerTestProvider]
        files_to_run_after.append(container_provider.after)
        raw_executable_paths.append("./" + container_provider.after.short_path)
        ctx.actions.run(
            executable=container_provider.before,
            outputs=[container_provider.out]
        )

    out_file = ctx.actions.declare_file("name.sh")
    ctx.actions.write(
        output =out_file,
        is_executable = True,
#        content = "\n".join(raw_executable_paths))
        content = "echo hello world")

    print("theoretically done starting dockerfiles here")
    runfiles = ctx.runfiles(files = files_to_run_after)
    print("RUNNING ACTUAL TEST HERE")

    ctx.actions.run(
        executable=ctx.attr.test[DefaultInfo].files_to_run.executable,
        outputs=ctx.attr.test.files.to_list(),
        inputs=[ctx.attr.test[DefaultInfo].files_to_run.runfiles_manifest]
    )

    print("done running external test")
    print(files_to_run_after)
    return DefaultInfo(executable=out_file)


n_run_test = rule (
    _run_test,
    attrs = {
        "containers" : attr.label_list(providers=[ContainerTestProvider]),
        "test" : attr.label(),
    },
    doc = "Compares two files, and fails if they are different.",
    test=True,
    outputs = {
        "out": "something.sh",
    }
)
