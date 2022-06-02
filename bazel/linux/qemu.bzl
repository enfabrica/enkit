load("//bazel/linux:runner.bzl", "CREATE_RUNNER_ATTRS", "create_runner")
load("@bazel_skylib//lib:shell.bzl", "shell")

DEFAULT_QEMU_FLAGS = [
    "-enable-kvm",
    "-cpu",
    "host",
    "-machine",
    "pc,accel=kvm,usb=off,dump-guest-core=off",
    "-m",
    "2048",
    "-smp",
    "4,sockets=4,cores=1,threads=1",
    "-rtc",
    "base=utc",
    "-boot",
    "strict=on",  # Only boot from devices specified, in the order specified.
    "-nographic",  # No UI, and stdio acts as ttyS0.
    "-no-reboot",  # When kernel crashes / exits, don't restart VM.
]

DEFAULT_KERNEL_FLAGS = [
    "rw",
    "console=ttyS0",  # early boot messages to serial, which qemu sends to stdio.
    "panic=-1",  # reboot (exit qemu) immediately on panic.
]

def _kernel_qemu_run(ctx):
    code = """
QEMU_FLAGS={qemu_flags}
QEMU_SEARCH={qemu_search}
KERNEL_FLAGS={kernel_flags}

for qemu in "${{QEMU_SEARCH[@]}}"; do
    QEMU_BINARY=$(which "$qemu") && break
done
test -n "$QEMU_BINARY" || {{
    echo 1>&2 "NO QEMU FOUND - Looked in: ${{QEMU_SEARCH[@]}}"
    echo 1>&2 "Use qemu_binary= or qemu_search= in the rule to tune the behavior"
    exit 10
}}

if [ -n "$ROOTFS" ]; then
    QEMU_FLAGS+=("-drive" "file=$ROOTFS,if=virtio,cache=none")
else
    QEMU_FLAGS+=("-fsdev" "local,security_model=none,multidevs=remap,id=fsdev-fsRoot,path=/")
    QEMU_FLAGS+=("-device" "virtio-9p-pci,fsdev=fsdev-fsRoot,mount_tag=/dev/root")
    KERNEL_FLAGS+=("root=/dev/root" "rootfstype=9p" "init=$INIT")
    KERNEL_FLAGS+=("rootflags=trans=virtio,version=9p2000.L,msize=5000000,cache=mmap,posixacl")
fi
test -z "$KERNEL" || QEMU_FLAGS+=("-kernel" "$KERNEL")
test -z "$SINGLE" || KERNEL_FLAGS+=("init=/bin/sh")

QEMU_FLAGS+=("-append" "${{KERNEL_FLAGS[*]}} ${{KERNEL_OPTS[*]}}")
QEMU_FLAGS+=("${{EMULATOR_OPTS[@]}}")

echo 1>&2 '$' "$QEMU_BINARY" "${{QEMU_FLAGS[@]}}"
if [ -z "$INTERACTIVE" -a -z "$SINGLE" ]; then
    "$QEMU_BINARY" "${{QEMU_FLAGS[@]}}" </dev/null | tee "$OUTPUT_FILE"
else
    "$QEMU_BINARY" "${{QEMU_FLAGS[@]}}"
fi
"""
    qemu_search = ctx.attr.qemu_search
    runfiles = None
    if ctx.attr.qemu_binary:
        di = ctx.attr.qemu_binary[DefaultInfo]
        qemu_search = [di.files_to_run.executable.short_path]
        runfiles = di.default_runfiles
    qemu_flags = ctx.attr.qemu_defaults + ctx.attr.qemu_flags

    kernel_flags = ctx.attr.kernel_defaults + ctx.attr.kernel_flags
    return create_runner(ctx, ctx.attr.archs, code, runfiles = runfiles, extra = {
        "qemu_search": shell.array_literal(qemu_search),
        "qemu_flags": shell.array_literal(qemu_flags),
        "kernel_flags": shell.array_literal(kernel_flags),
    })

kernel_qemu_run = rule(
    doc = """Runs code in a qemu instance.

The code to run is specified by using the "runner" attribute, which
pretty much provides a self contained directory with an init script.
See the RuntimeBundleInfo provider for details.
""",
    implementation = _kernel_qemu_run,
    executable = True,
    attrs = dict(
        CREATE_RUNNER_ATTRS,
        **{
            "archs": attr.string_list(
                default = ["host", "x86_64"],
                doc = "Architectures supported by this test",
            ),
            "kernel_defaults": attr.string_list(
                default = DEFAULT_KERNEL_FLAGS,
                doc = "Default parameters passed on the kernel command line",
            ),
            "kernel_flags": attr.string_list(
                doc = "Additional flags to pass to the kernel. Appended to kernel_defaults",
            ),
            "qemu_binary": attr.label(
                doc = "A target defining the qemu binary to run. If unspecified, it will use a search path",
                executable = True,
                cfg = "target",
            ),
            "qemu_search": attr.string_list(
                doc = "Qemu binaries to try to run, in turn, until one is found. Ignored if qemu_binary is specified.",
                default = ["qemu-system-x86_64", "qemu"],
            ),
            "qemu_defaults": attr.string_list(
                doc = "Default flags to pass to qemu. Use only if you need to change the defaults",
                default = DEFAULT_QEMU_FLAGS,
            ),
            "qemu_flags": attr.string_list(
                doc = "Additional flags to pass to qemu. Appended to the default flags",
                default = [],
            ),
        }
    ),
)
