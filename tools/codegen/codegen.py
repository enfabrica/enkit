#!/usr/bin/python3
#
# Render a jinja2 template using a variety of input data formats.
#
# Data inputs are parsed in the order they are specified on the commandline.
# Data is merged in the following ways:
#   dictionaries are merged, recursively.
#   lists are appended to each other.
#   scalars override the previous value.
#
# Currently, we are using jinja2 version 3.1.1, which is documented here:
#  https://jinja.palletsprojects.com/en/3.1.x/

# standard libraries
import inspect
import json
import os
import re
import subprocess
import sys
import zipfile
from typing import Dict, List

# third party libraries
import jinja2
import jinja2.ext
import jsonschema
import yaml
from absl import app, flags, logging

from tools.codegen import data_loader

FLAGS = flags.FLAGS
flags.DEFINE_multi_string("load", [], "List of data files to load.")
flags.DEFINE_multi_string("override", [], "List of key=value pair data.")
flags.DEFINE_string("schema", None, "JSON schema to check against.")
flags.DEFINE_multi_string("output", None, "Output files to generate.")
flags.DEFINE_boolean("to_stdout", False, "Writes all output to stdout.")
flags.DEFINE_multi_string("incdir", [], "Paths to search for template files.")
flags.DEFINE_boolean(
    "multigen_mode",
    False,
    "Generates a zip file containing a file for each data context index.",
)


class RaisedError(jinja2.TemplateError):
    def __init__(self, message=None):
        super().__init__(message)


class RaiseExtension(jinja2.ext.Extension):
    tags = {"raise"}

    def parse(self, parser):
        ln = next(parser.stream).lineno
        message = parser.parse_expression()
        return jinja2.nodes.CallBlock(
            self.call_method("_raise", [message], lineno=ln), [], [], [], lineno=ln
        )

    def _raise(self, msg, caller):
        raise RaisedError(msg)


def bitwise_and_function(a, b):
    return a & b


def bitwise_or_function(a, b):
    return a | b


def bitwise_xor_function(a, b):
    return a ^ b


def bitwise_not_function(a):
    return ~a


def re_split_function(regex, text):
    return re.split(regex, text)


def re_sub_function(text, match, sub):
    return re.sub(match, sub, text)


def _merge(a, b, path=None):
    """Merges structure b into structure a."""
    if path is None:
        path = []
    if isinstance(b, (str, int, float)):
        a = b
    elif isinstance(a, dict) and isinstance(b, dict):
        for key in b:
            if key in a:
                a[key] = _merge(a[key], b[key], path + [key])
            else:
                a[key] = b[key]
    elif isinstance(a, list) and isinstance(b, list):
        a += b
    else:
        raise TypeError(f"Could not merge type {type(a)!r} with type {type(b)!r}.")
    return a

def log_filter(text):
    for frameinfo in inspect.stack():
        template = frameinfo.frame.f_globals.get("__jinja_template__")
        if template is not None:
            break
    lineno = 0
    filename = "?"
    if template is not None:
        filename = template.filename
        lineno = template.get_corresponding_lineno(inspect.currentframe().f_back.f_lineno)
        logging.info(f"{filename}:{lineno}: {text}")
    else:
        logging.info(f"unknown source: {text}")
    return ''

class Template(data_loader.DataLoader):
    def __init__(self, other=None):
        super(Template, self).__init__()
        search_paths = ["."] + FLAGS.incdir
        self.env = jinja2.Environment(
            extensions=[
                "jinja2.ext.do",
                "jinja2.ext.loopcontrols",
                "jinja2.ext.debug",
                "jinja2_strcase.StrcaseExtension",
                RaiseExtension,
            ],
            loader=jinja2.FileSystemLoader(search_paths),
            keep_trailing_newline=True,
            autoescape=False,
        )
        self.env.globals.update(
            {
                "bitwise_and": bitwise_and_function,
                "bitwise_or": bitwise_or_function,
                "bitwise_xor": bitwise_xor_function,
                "bitwise_not": bitwise_not_function,
                "re_split": re_split_function,
                "re_sub": re_sub_function,
            }
        )
        self.env.filters.update({
            "re_sub": re_sub_function,
            "log": log_filter,
        })
        self.context = {"_DATA": [], "_TEMPLATE": ""}
        self.template = None
        self.template_path = None
        if other:
            self.context = other.context
            self.template = other.template
            self.template_path = other.template_path

    def GetContextKeys(self):
        return self.context.keys()

    def GetSubcontext(self, key):
        t = Template(self)
        t.context = {}
        if "_default" in self.context:
            t.context = _merge(t.context, self.context["_default"])
        t.context = _merge(t.context, self.context[key])
        return t

    def Override(self, override: str):
        k, v = override.split("=", 2)
        self.context = _merge(self.context, {k: v})

    def LoadDataFile(self, path: str):
        # TODO(jonathan): support protobuffer?
        # TODO(jonathan): support toml?
        ext = os.path.splitext(path)[-1]
        if ext == ".json":
            self.LoadJsonFile(path)
        elif ext == ".yaml" or ext == ".pkgdef":
            self.LoadYamlFile(path)
        else:
            logging.error("Unsupported data file extension %r: %r", ext, path)

    def LoadJsonFile(self, path: str):
        with open(path, "r") as fd:
            d = json.load(fd)
            self.context = _merge(self.context, d)
            self.context["_DATA"] += [path]
            logging.vlog(1, "Loaded data from %r", path)
            logging.vlog(2, "%r", d)

    def LoadYamlFile(self, path):
        with open(path, "r") as fd:
            d = yaml.safe_load(fd)
            self.context = _merge(self.context, d)
            self.context["_DATA"] += [path]
            logging.vlog(1, "Loaded data from %r", path)
            logging.vlog(2, "%r", d)

    def FixToplevelNames(self):
        """Normalize the toplevel keys in the data context.

        Jinja2 requires that all top-level keys in the data context have
        simple Python-compatible names.  This routine searches for keys
        with names like "$defs" and turns them into "_DOLLAR_defs".

        We also create a reference to the top-level context and name
        that context "_TOP".
        """
        keys = list(self.context.keys())  # because we modify keys.
        for key in keys:
            if "$" in key:
                alternate = key.replace("$", "_DOLLAR_")
                self.context[alternate] = self.context[key]
                del self.context[key]
        self.context["_TOP"] = self.context

    def LoadTemplate(self, path):
        logging.vlog(1, "Loading template %r", path)
        self.template_path = path
        self.template = self.env.get_template(path)

    def CheckSchema(self):
        # Raises jsonschema.exceptions.{ValidationError,SchemaError}
        if not FLAGS.schema:
            return
        with open(FLAGS.schema, "r") as fd:
            schema = None
            if FLAGS.schema.endswith(".yaml"):
                schema = yaml.safe_load(fd)
            else:
                schema = json.load(fd)
            logging.vlog(1, "Loaded schema %r", FLAGS.schema)
            jsonschema.validate(instance=self.context, schema=schema)

    def Render(self):
        self.CheckSchema()  # raises exception on error.
        text = self.template.render(self.context)
        text = re.sub(r"(?m)[ \t]+$", "", text)  # remove trailing whitespace
        return text

    def InferOutputFile(self):
        output_file = os.path.basename(self.template_path)
        for extension in (".jinja", ".jinja2", ".template", ".tmpl"):
            if output_file.endswith(extension):
                output_file = output_file[: -len(extension)]
        if output_file == os.path.basename(self.template_path):
            output_file += ".out"
        return output_file

    def RenderToOutput(self, output_file=None):
        if output_file:
            self.context["_OUTPUT_PATH"] = output_file
            self.context["_OUTPUT_FILE"] = os.path.basename(output_file)
        output = self.Render()
        if FLAGS.to_stdout:
            sys.stdout.write(output)
            logging.vlog(1, "Wrote %d bytes to stdout", len(output))
        elif FLAGS.multigen_mode:
            if len(FLAGS.output) != 1:
                logging.error("Only one output zip file can be specified in multimode.")
                sys.exit(1)
            with zipfile.ZipFile(FLAGS.output[0], mode="a") as zf:
                zf.writestr(output_file, output)
                zf.close()
        else:
            with open(output_file, "w") as fd:
                fd.write(output)
                fd.close()
            logging.vlog(1, "Wrote %d bytes to %r", len(output), output_file)


def main(argv):
    context = Template()
    for path in FLAGS.load:
        context.LoadDataFile(path)
    for override in FLAGS.override:
        context.Override(override)
    context.FixToplevelNames()
    template_files = argv[1:]
    if len(template_files) > 1 and FLAGS.output:
        logging.error("You cannot specify --output files when multiple templates are present")
        sys.exit(1)
    for path in template_files:
        t = Template(context)
        t.LoadTemplate(path)
        if FLAGS.multigen_mode:
            logging.vlog(1, "Is in multigen mode")
            for k in t.GetContextKeys():
                if k.startswith("_"):
                    continue
                subt = t.GetSubcontext(k)
                subt.RenderToOutput(k)
        elif FLAGS.output:
            for output in FLAGS.output:
                logging.vlog(1, "Output = %r", output)
                t.RenderToOutput(output)
        else:
            logging.vlog(1, "Output is inferred")
            t.RenderToOutput(t.InferOutputFile())


if __name__ == "__main__":
    app.run(main)
