#!/usr/bin/env python3

"""This is a replacement for the functionality of the @enkit/bazel/astore/astore_upload_file.sh
Original file will only have arguments parsing and passing them down to this script.
"""

# standard libraries
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile

# third party libraries
from absl import app, flags
from absl import logging as log
from python import runfiles

FLAGS = flags.FLAGS

# Define command-line flags
flags.DEFINE_string(
    "astore_base_path", None, "Remote file name where to store all files"
)
flags.DEFINE_multi_string("upload_file", None, "Files to upload")
flags.DEFINE_string("uidfile", None, "File to update with UID information")
flags.DEFINE_multi_string("tag", None, "Tags to add to the upload")
# FIXME: technically it's just a switch, only json data will be updated with the local_file
#        anything else just passed as is.
flags.DEFINE_enum("output_format", "table", ["table", "json"], "Output format")



def sha256sum(filename):
    """Calculate SHA256 hash of a file."""
    h = hashlib.sha256()
    with open(filename, "rb") as f:
        for block in iter(lambda: f.read(4096), b""):
            h.update(block)
    return h.hexdigest()


def update_starlark_version_file(uidfile, fname, file_uid, file_sha):
    """Update UID and SHA variables in the build file."""
    if not os.path.isfile(uidfile):
        log.error("Error: %s: file not found", uidfile)
        sys.exit(3)

    uidfile = os.path.realpath(uidfile)
    varname = os.path.basename(fname).translate(
        str.maketrans(
            "abcdefghijklmnopqrstuvwxyz",
            "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
            "".join(
                c for c in map(chr, range(256)) if not c.isalnum() and c not in "\r\n"
            ),
        )
    )

    uid_varname = f"UID_{varname}"
    sha_varname = f"SHA_{varname}"

    with open(uidfile, "r", encoding="utf-8") as f:
        content = f.read()

    new_content = re.sub(
        f'^{uid_varname} = ".*"',
        f'{uid_varname} = "{file_uid}"',
        content,
        flags=re.MULTILINE,
    )
    new_content = re.sub(
        f'^{sha_varname} = ".*"',
        f'{sha_varname} = "{file_sha}"',
        new_content,
        flags=re.MULTILINE,
    )

    with open(uidfile, "w", encoding="utf-8") as f:
        f.write(new_content)

    # Verify the update was successful
    with open(uidfile, "r", encoding="utf-8") as f:
        if not re.search(f'^{uid_varname} = "{file_uid}"', f.read(), re.MULTILINE):
            log.error("Error: failed to update %s in %s", uid_varname, uidfile)
            log.error("       Is this variable missing from this file?")
            sys.exit(5)

    log.info("Updated %s in %s", uid_varname, uidfile)


def main(argv):
    del argv

    r = runfiles.Runfiles.Create()
    astore_client = r.Rlocation("net_enfabrica_binary_astore/file/downloaded")

    # questionable
    # if FLAGS.upload_file and len(FLAGS.upload_file) > 1 and FLAGS.uidfile:
    #     log.fatal("Error: cannot update uidfile when uploading multiple files")

    if not FLAGS.upload_file:
        log.fatal("Error: no files to upload")

    if not FLAGS.astore_base_path:
        log.fatal("Error: no astore base path specified")

    log.info("Processing files: %s", FLAGS.upload_file)

    # Create temporary file for metadata
    with tempfile.NamedTemporaryFile(
        prefix="astore.", suffix=".json", delete=False
    ) as temp:
        temp_json = temp.name

    try:
        astore_cmd = [astore_client, "upload"]

        json_data = dict()
        # Process each local_file sequentially
        for local_file in FLAGS.upload_file:
            cmd = astore_cmd.copy()

            if FLAGS.tag:
                cmd.extend(f"--tag={t}" for t in FLAGS.tag)

            cmd.extend(
                [
                    "--disable-git",
                    "--file",
                    FLAGS.astore_base_path,
                    "--meta-file",
                    temp_json,
                    "--console-format",
                    FLAGS.output_format,
                    local_file,
                ]
            )
            log.info("Running command: %s", cmd)

            # Run the upload command
            result = subprocess.run(cmd, capture_output=True, text=True)
            if result.returncode != 0:
                log.error("Error uploading %s: %s", local_file, result.stderr)
                sys.exit(1)

            # Print output based on format
            if FLAGS.output_format == "json":
                data = json.loads(result.stdout)
                data["Artifacts"][0]["Target"] = local_file
                if not json_data:
                    json_data = data
                else:
                    json_data["Artifacts"].append(data["Artifacts"][0])
            else:
                print(result.stdout)

            # Update build file if specified
            if FLAGS.uidfile:
                # Extract UID from metadata file
                with open(temp_json, "r", encoding="utf-8") as f:
                    json_data = json.load(f)

                file_uid = json_data["Artifacts"][0]["Uid"]
                if not file_uid:
                    log.error(
                        "Error: no UID found for %s uploaded as %s",
                        local_file,
                        FLAGS.file,
                    )
                    sys.exit(2)

                update_starlark_version_file(
                    FLAGS.uidfile, local_file, file_uid, sha256sum(local_file)
                )

        if FLAGS.output_format == "json":
            print(json.dumps(json_data))
    finally:
        # Clean up temporary file
        if os.path.exists(temp_json):
            os.unlink(temp_json)


if __name__ == "__main__":
    flags.mark_flags_as_required(["astore_base_path", "upload_file"])

    app.run(main)
