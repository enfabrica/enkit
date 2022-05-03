#!/usr/bin/python3
# -*- coding: utf-8 -*-
#
# Usage: jinjafmt [files...]
#
# jinjafmt reformats a jinja2 template file to be more readable, without changing
# the function of the template in any way.  It operates in-place on all supplied
# filenames.  If no filenames are provided, it reads a template from stdin and
# writes a reformatted template on stdout.
#
# Formatting changes:
#  *  Trailing whitespace is explicitly labelled with {# ws #} comments.
#  *  All statements are placed on their own line, with {%- in column 0.
#  *  All statements are indented according to scoping depth.
#  *  All statements are converted to be left-gobbling: {%- statement %}, and
#     {# blank line #} comments are inserted where needed.
#  *  Some unnecessary whitespace is removed.
#
# If run with the "--check" option, jinjafmt reports if changes to a template
# file would be made, without making any changes.

# standard libraries
import re
import sys

# third party libraries
from absl import app, flags

FLAGS = flags.FLAGS
flags.DEFINE_boolean("check", False, "Just check if changes would be made.")


def Reformat(text: str) -> str:
    """Reformat reformats a jinja2 template file's text."""
    # Turn "^{% foo" into "{# blank line #}\n{%- foo"
    text = re.sub(r"^ {% (?!-)", r"{# blank line #}\n{%-", text, flags=re.MULTILINE | re.VERBOSE)
    # Convert all non-left-gobbling statements into left-gobbling:
    text = re.sub(r" ([^\n]+?) {% (?!-) ", r"\1\n{%-", text, flags=re.MULTILINE | re.VERBOSE)
    # Eliminate whitespace before left-gobbling statements
    text = re.sub(r" [\ \t]+ {%\s*- ", r"{%-", text, flags=re.MULTILINE | re.VERBOSE)
    # Put control statements on their own line
    text = re.sub(r"^ (.+) {%-", r"\1\n{%-", text, flags=re.MULTILINE | re.VERBOSE)
    text = re.sub(r" ( \{%.*?%\} (?: [\ \t]* \{\#.*?\#\} )? ) ((?!\s*\{\#).+) $", r'\1\n{{- ""}}\2', text, flags=re.MULTILINE | re.VERBOSE)
    # Label all trailing whitespace as intentional:
    text = re.sub(r"([\ \t]+) $", r"\1{# ws #}", text, flags=re.MULTILINE | re.VERBOSE)
    # Remove needless whitespace:
    text = re.sub(r"{%\s*-", r"{%-", text)
    text = re.sub(r"\s*(-?)\s*%}", r" \1%}", text)
    # Re-indent all control statements:
    indent = 0
    chunks = re.split(r"(\{%.+?%\})", text, flags=re.MULTILINE | re.VERBOSE)
    for i in range(len(chunks)):
        mo = re.match(r"{%\s*-\s*(\w+)(.*)", chunks[i])
        if mo:
            cmd = mo.group(1)
            if cmd in ["endfor", "endif", "else", "elif", "endmacro", "endcall", "endfilter", "endset", "endblock"]:
                indent -= 1
            chunks[i] = r"{%%- %s%s%s" % ("  " * indent, mo.group(1), mo.group(2))
            if cmd in ["for", "if", "else", "elif", "macro", "call", "filter", "block"]:
                indent += 1
            # special case: {# set foo #}...{# endset #}
            if re.match(r"\{%\s*-?\s*set\s+\w+\s*-?\s*%\}.*", chunks[i], flags=re.VERBOSE):
                indent += 1
    text = "".join(chunks)
    return text


def ReformatFile(infile: str, outfile: str):
    """Reformats a file in place.

    Returns 0, unless in "--check" mode and a file needs changes.
    """
    with open(infile, "r") as fd:
        text = fd.read()
        fd.close()

    newtext = Reformat(text)

    if FLAGS.check:
        if newtext != text:
            print("%r: Reformatting needed.")
            return 1

    with open(outfile, "w") as fd:
        fd.write(newtext)
        fd.close()
    return 0


def main(argv):
    if argv[1:]:
        failed_checks = 0
        for f in argv[1:]:
            failed_checks += ReformatFile(f, f)
            print("processed %r" % f)
        return failed_checks
    else:
        text = sys.stdin.read()
        newtext = Reformat(text)
        if FLAGS.check:
            if newtext != text:
                print("Reformatting needed.")
                return 1
        sys.stdout.write(newtext)
        return 0


if __name__ == "__main__":
    app.run(main)
