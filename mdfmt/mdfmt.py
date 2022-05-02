# -*- coding: utf-8 -*-
"""Reformat markdown.

A simple wrapper for the mdformat library.
"""

# third party libraries
import mdformat
from absl import app, flags, logging

FLAGS = flags.FLAGS
flags.DEFINE_bool("check", False, "Enable check mode.")
flags.DEFINE_string("output", None, "Write reformatted text here (instead of operating in place).")


def format_file(in_fname, out_fname, check=False):
    with open(in_fname, "r", encoding="utf-8") as fd:
        unformatted = fd.read()
    extensions = {"gfm", "tables"}
    options = {"wrap": 78}
    formatted = mdformat.text(unformatted, options=options, extensions=extensions)
    if check:
        return formatted == unformatted
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
