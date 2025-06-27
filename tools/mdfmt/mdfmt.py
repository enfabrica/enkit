# -*- coding: utf-8 -*-
"""Reformat markdown.

A simple wrapper for the mdformat library.

Usage:

    # stream stdin to stdout:
    bazel run //tools/mdftm -- -

    # operate on a set of files in place:
    bazel run //tools/mdfmt -- <files>

    # operate on a file with a different output file:
    bazel run //tools/mdfmt -- <file_in> --output <file_output>

    # check if files need to be reformatted
    bazel run //tools/mdfmt -- --check <files>

"""

import sys

# third party libraries
import mdformat
from absl import app, flags, logging

FLAGS = flags.FLAGS
flags.DEFINE_bool("check", False, "Enable check mode.")
flags.DEFINE_string("output", None, "Write reformatted text here (instead of operating in place).")


def format_file(in_fname, out_fname, check=False):
    unformatted = None
    if in_fname == "-":
        unformatted = sys.stdin.read()
    else:
        with open(in_fname, "r", encoding="utf-8") as fd:
            unformatted = fd.read()
    extensions = {"gfm", "tables"}
    options = {"wrap": 78}
    formatted = mdformat.text(unformatted, options=options, extensions=extensions)
    if check:
        return formatted == unformatted
    else:
        if out_fname == "-":
            print(formatted)
        else:
            with open(out_fname, "w", encoding="utf-8") as fd:
                fd.write(formatted)
        return True


def main(argv):
    errors = 0
    if FLAGS.check:
        verb = "checking"
    else:
        verb = "reformatting"
    for f in argv[1:]:
        logging.vlog(1, "%s %r", verb, f)
        outf = f
        if FLAGS.output:
            outf = FLAGS.output
        if not format_file(f, outf, FLAGS.check):
            logging.error("Error %s %r", verb, f)
            errors += 1
    return errors


if __name__ == "__main__":
    app.run(main)
