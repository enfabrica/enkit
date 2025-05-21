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
from absl import logging as logger

FLAGS = flags.FLAGS

# Define command-line flags
flags.DEFINE_string("file", "", "Remote file name where to store all files")
flags.DEFINE_string("uidfile", "", "File to update with UID information")
flags.DEFINE_string("upload_tag", "", "Tag to add to the upload")
# FIXME: technically it's just a switch, only json data will be updated with the local_file
#        anything else just passed as is.
flags.DEFINE_enum("output_format", "table", ["table", "json"], "Output format")
flags.DEFINE_string("astore", "astore", "Path to astore binary")


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
        logger.error("Error: %s: file not found", uidfile)
        sys.exit(3)

    uidfile = os.path.realpath(uidfile)
    varname = os.path.basename(fname).translate(
        str.maketrans(
            "abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "".join(c for c in map(chr, range(256)) if not c.isalnum() and c not in "\r\n")
        )
    )

    uid_varname = f"UID_{varname}"
    sha_varname = f"SHA_{varname}"

    with open(uidfile, "r", encoding="utf-8") as f:
        content = f.read()

    new_content = re.sub(f'^{uid_varname} = ".*"', f'{uid_varname} = "{file_uid}"', content, flags=re.MULTILINE)
    new_content = re.sub(f'^{sha_varname} = ".*"', f'{sha_varname} = "{file_sha}"', new_content, flags=re.MULTILINE)

    with open(uidfile, "w", encoding="utf-8") as f:
        f.write(new_content)

    # Verify the update was successful
    with open(uidfile, "r", encoding="utf-8") as f:
        if not re.search(f'^{uid_varname} = "{file_uid}"', f.read(), re.MULTILINE):
            logger.error("Error: failed to update %s in %s", uid_varname, uidfile)
            logger.error("       Is this variable missing from this file?")
            sys.exit(5)

    logger.info("Updated %s in %s", uid_varname, uidfile)


def main(argv):
    # Get file list from command line arguments (after the script name)
    local_files_list = argv[1:]

    # Check if we have any files to process
    if not local_files_list:
        logger.error("No local files to upload specified. Please provide files as command line arguments.")
        sys.exit(1)

    if len(local_files_list) > 1 and FLAGS.uidfile:
        logger.error("Error: cannot update uidfile when uploading multiple files")
        sys.exit(1)

    logger.info("Processing files: %s", local_files_list)

    # Create temporary file for metadata
    with tempfile.NamedTemporaryFile(prefix="astore.", suffix=".json", delete=False) as temp:
        temp_json = temp.name

    try:
        if FLAGS.astore == "astore":
            astore_cmd = ["enkit", "astore", "upload"]
        else:
            astore_cmd = [FLAGS.astore, "upload"]

        json_data = dict()
        # Process each local_file sequentially
        for local_file in local_files_list:
            cmd = astore_cmd.copy()

            if FLAGS.upload_tag:
                # FIXME astore_upload_file.sh has -t provided by the bazel rule
                cmd.extend(FLAGS.upload_tag.split())

            cmd.extend(["--disable-git", "--file", FLAGS.file, "--meta-file", temp_json, "--console-format", FLAGS.output_format, local_file])
            logger.info("Running command: %s", cmd)

            # Run the upload command
            result = subprocess.run(cmd, capture_output=True, text=True)
            if result.returncode != 0:
                logger.error("Error uploading %s: %s", local_file, result.stderr)
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
                    logger.error("Error: no UID found for %s uploaded as %s", local_file, FLAGS.file)
                    sys.exit(2)

                update_starlark_version_file(FLAGS.uidfile, local_file, file_uid, sha256sum(local_file))

        if FLAGS.output_format == "json":
            print(json.dumps(json_data))
    finally:
        # Clean up temporary file
        if os.path.exists(temp_json):
            os.unlink(temp_json)


if __name__ == "__main__":
    app.run(main)
