#!/usr/bin/python3
"""Extracts metadata from a bzl file.

This script parses a .bzl file and produces a data structure representation
suitable for rendering into documentation.
"""

# standard libraries
import ast
import os.path
import sys
import textwrap
import typing

# third party libraries
import yaml
from absl import app, flags, logging

FLAGS = flags.FLAGS
flags.DEFINE_string("input", None, "bzl file to parse")
flags.DEFINE_string("short_path", None, 'bazel "short path" to bzl file')
flags.DEFINE_string("output", None, "yaml file to write")


def _trim_docstring(docstring):
    # From PEP 257
    if not docstring:
        return ""
    # Convert tabs to spaces (following the normal Python rules)
    # and split into a list of lines:
    lines = docstring.expandtabs().splitlines()
    # Determine minimum indentation (first line doesn't count):
    indent = 1000
    for line in lines[1:]:
        stripped = line.lstrip()
        if stripped:
            indent = min(indent, len(line) - len(stripped))
    # Remove indentation (first line is special):
    trimmed = [lines[0].strip()]
    if indent < 1000:
        for line in lines[1:]:
            trimmed.append(line[indent:].rstrip())
    # Strip off trailing and leading blank lines:
    while trimmed and not trimmed[-1]:
        trimmed.pop()
    while trimmed and not trimmed[0]:
        trimmed.pop(0)
    # Return a single string:
    s = "\n".join(trimmed)

    blocks = s.split("\n\n")
    for i in range(len(blocks)):
        if blocks[i].startswith("Args:"):
            blocks[i] = args_block_to_markdown(blocks[i])
    s = "\n\n".join(blocks)

    return s


def _normalize_str(s):
    s = s.strip("\r\n")
    s = textwrap.dedent(s)

    return s


def args_block_to_markdown(block):
    lines = block.splitlines()
    text = ["### Args:", ""]
    base_indent = len(lines[1]) - len(lines[1].lstrip())
    for line in lines[1:]:
        indent = len(line) - len(line.lstrip())
        if indent == base_indent:
            text.append("* " + line.lstrip())
        else:
            text.append("  " + line.lstrip())
    return "\n".join(text)


def _normalize(node):
    """Recursively converts AST nodes to plain old data."""
    if isinstance(node, ast.Dict):
        d = {}
        for k, v in zip(node.keys, node.values):
            d[_normalize(k)] = _normalize(v)
        return d
    elif isinstance(node, ast.Constant):
        return _normalize(node.value)
    elif isinstance(node, ast.List):
        return [_normalize(x) for x in node.elts]
    elif isinstance(node, ast.Attribute):
        return f"{_normalize(node.value)}.{_normalize(node.attr)}"
    elif isinstance(node, ast.Name):
        return _normalize(node.id)
    elif isinstance(node, ast.Call):
        d = {}
        d["name"] = _normalize(node.func)
        d["args"] = _normalize(node.args)
        d["kwargs"] = {_normalize(n.arg): _normalize(n.value) for n in node.keywords}
        return d
    elif isinstance(node, typing.List):
        return [_normalize(x) for x in node]
    elif isinstance(node, typing.Dict):
        return {_normalize(k): _normalize(v) for k, v in node.iter()}
    elif isinstance(node, ast.arguments):
        return _normalize(node.args)
    elif isinstance(node, ast.arg):
        return _normalize(node.arg)
    elif isinstance(node, str):
        return _normalize_str(node)
    elif isinstance(node, (float, int, bool)):
        return node
    else:
        logging.error("Unhandled node %r", node)
        return "*"


class BzlDoc(object):
    """BzlDoc extracts metadata from bzl files, and renders documentation."""

    def __init__(self):
        self.data = {
            "rules": {},
            "macros": {},
        }

    def ParseFile(self, filename):
        self.filename = filename
        with open(filename, "r", encoding="utf-8") as fd:
            text = fd.read()
            fd.close()
        self.data["filename"] = os.path.basename(filename)
        if FLAGS.short_path:
            self.data["short_path"] = FLAGS.short_path
            self.data["label"] = f"//{os.path.dirname(FLAGS.short_path)}:{os.path.basename(FLAGS.short_path)}"
        self.ParseText(text)

    def ParseText(self, text):
        module = ast.parse(text)
        docstr = ast.get_docstring(module)
        if docstr:
            self.data["doc"] = _trim_docstring(docstr)
        logging.vlog(1, ast.dump(module))
        # Find rules
        for node in ast.walk(module):
            if isinstance(node, ast.Assign):
                if isinstance(node.value, ast.Call):
                    if isinstance(node.value.func, ast.Name):
                        if node.value.func.id == "rule":
                            self.data["rules"][_normalize(node.targets[0])] = _normalize(node.value)

        # Find macros
        for node in ast.walk(module):
            if isinstance(node, ast.FunctionDef):
                if not node.name.startswith("_"):
                    macro = {
                        "name": node.name,
                        "args": _normalize(node.args),
                    }
                    docstr = ast.get_docstring(node)
                    if docstr:
                        # TODO(jonathan): parse python docstrings.
                        macro["doc"] = _trim_docstring(docstr)
                    self.data["macros"][node.name] = macro

    def Write(self, fd):
        opts = {"width": 78, "indent": 2, "sort_keys": False}
        yaml.safe_dump(self.data, fd, **opts)

    def WriteFile(self, filename):
        with open(filename, "w", encoding="utf-8") as fd:
            self.Write(fd)


def main(argv):
    if len(argv) != 1:
        logging.fatal("Unsupported command line arguments: %r", argv[1:])

    bzldoc = BzlDoc()
    bzldoc.ParseFile(FLAGS.input)
    if FLAGS.output:
        bzldoc.WriteFile(FLAGS.output)
    else:
        bzldoc.Write(sys.stdout)


if __name__ == "__main__":
    app.run(main)
