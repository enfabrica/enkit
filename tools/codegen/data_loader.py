"""data_loader.DataLoader facilitates the loading of JSON and YAML files.

DataLoader ensures that data structures from separate files are combined in a
sane way:
    - dictionaries are recursively merged.
    - lists are concatenated.

The default YAML behavior is modified so that if the same key is encountered
twice in the same file, the elements of the keys are combined as if they had
been merged from separate data files.
"""
# standard libraries
import json
import os
import sys
import typing

# third party libraries
import yaml
from absl import logging

MERGEABLE = typing.Union[str, int, float, typing.List, typing.Dict]


def _merge(a: MERGEABLE, b: MERGEABLE, path: str = None) -> MERGEABLE:
    """Merges structure b into structure a."""
    if path is None:
        path = []
    if isinstance(b, (str, int, float)):
        a = b
    elif isinstance(a, typing.Dict) and isinstance(b, typing.Dict):
        for key in b:
            if key in a:
                a[key] = _merge(a[key], b[key], path + [key])
            else:
                a[key] = b[key]
    elif isinstance(a, typing.List) and isinstance(b, typing.List):
        a += b
    else:
        raise TypeError(f"Could not merge {type(a)} with {type(b)}.")
    return a


# https://stackoverflow.com/questions/44904290/getting-duplicate-keys-in-yaml-using-python
class _MergingLoader(yaml.CSafeLoader):
    pass


def _seq_constructor(loader: yaml.Loader, node, deep: bool = False):
    """I don't know why I have to define my own sequence constructor,
    I just know this doesn't work right if I don't."""
    if not isinstance(node, yaml.resolver.SequenceNode):
        raise yaml.resolver.ConstructorError(None, None, f"Expected a sequence node but found {node.id}", node.start_mark)
    seq = [loader.construct_object(child, deep=deep) for child in node.value]
    return seq


def _map_constructor(loader: yaml.Loader, node, deep: bool = False):
    mapping = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        val = loader.construct_object(value_node, deep=deep)
        mapping = _merge(mapping, {key: val})
    return mapping


_MergingLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, _map_constructor)
_MergingLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_SEQUENCE_TAG, _seq_constructor)


class DataLoader(object):
    """Facilitates the loading of JSON and YAML data files."""

    def __init__(self):
        super().__init__()  # needed for co-operative subclassing.
        self.context = {
            "_DATA": [],
        }

    def LoadDataFile(self, path: str):
        # TODO(jonathan): support protobuffer?
        # TODO(jonathan): support toml?
        ext = os.path.splitext(path)[-1]
        if ext.lower() == ".json":
            self.LoadJsonFile(path)
        elif ext.lower() == ".yaml" or ext.lower() == ".pkgdef":
            self.LoadYamlFile(path)
        else:
            raise Exception(f"Unsupported data file extension {ext!r}: {path!r}")

    def LoadJsonFile(self, path: str):
        with open(path, "r", encoding="utf-8") as fd:
            d = json.load(fd)
            self.context = _merge(self.context, d)
            self.context["_DATA"] += [path]
            logging.vlog(2, "Loaded data from %r", path)
            logging.vlog(3, "%r", d)

    def ParseYaml(self, text: str):
        try:
            d = yaml.load(text, Loader=_MergingLoader)
        except yaml.parser.ParserError as e:
            logging.error(e)
            sys.exit(1)
        logging.vlog(3, "Loaded: %r", d)
        self.context = _merge(self.context, d)

    def LoadYamlFile(self, path: str):
        with open(path, "r", encoding="utf-8") as fd:
            text = fd.read()
            self.ParseYaml(text)
            self.context["_DATA"].append(path)
            logging.vlog(2, "Loaded data from %r", path)
