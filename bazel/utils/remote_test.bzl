load("//bazel/utils:remote.bzl", "remote_run")
load("//bazel/utils:merge_kwargs.bzl", "merge_kwargs")
load("//bazel/utils:diff_test.bzl", "diff_test")

def remote_run_test(**remote_run_opts):
    name = remote_run_opts["name"]
    rsyncfile = name + ".rsync"
    sshfile = name + ".ssh"

    remote_run(**merge_kwargs(remote_run_opts, dict(
        rsync_cmd = "./bazel/utils/save-argv " + rsyncfile + " --files-from={filelist}",
        ssh_cmd = "./bazel/utils/save-argv " + sshfile,
        tools = ["//bazel/utils:save-argv"],
    )))

    native.genrule(
        name = name + "-argv",
        srcs = [],
        outs = [rsyncfile, sshfile],
        cmd = """
  script="$(location {name})"
  filename=$$(basename "$$script")
  dirname=$$(dirname "$$script")
  export OUTDIR="$$(mktemp -d)"
  set -e
  (cd ./"$$script.runfiles/enkit" && ../../"$$filename") && (
    cp "$$OUTDIR"/{rsyncfile} $(location {rsyncfile}) &>/dev/null || touch $(location {rsyncfile})
    cp "$$OUTDIR"/{sshfile} $(location {sshfile}) &>/dev/null || touch $(location {sshfile})
  )
  """.format(name = name, rsyncfile = rsyncfile, sshfile = sshfile),
        tools = [":" + name],
    )

    # Check that the command line for rsync, ssh, and the list of files is correct.
    diff_test(
        name = name + "_argv_rsync_test",
        actual = ":" + rsyncfile,
        expected = "testdata/remote/" + rsyncfile + ".expected",
    )

    diff_test(
        name = name + "_argv_ssh_test",
        actual = ":" + sshfile,
        expected = "testdata/remote/" + sshfile + ".expected",
    )

    diff_test(
        name = name + "_filelist_test",
        actual = ":" + name + ".files_to_copy",
        expected = "testdata/remote/" + name + ".files_to_copy.expected",
    )
