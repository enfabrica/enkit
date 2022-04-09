"""A tool to merge multiple app.yaml files together.

To run, just execute:
  
  python3 ./merge.py input1.yaml override.yaml [override.yaml...]

It will output the merged configuration file on stadout.

The tool provides a few flags to apply transformations to the
generated file. Use: "python3 ./merge.py -h" to see the help
messages.

Exits with a 0 status on success. Errors are output on stderr.
"""
import argparse
import sys
import yaml
import copy

def handler_ix(dest: list, el) -> int:
  """Returns the index of element in dest, with some app.yaml awareness.
  
  Specifically, it checks if el has the attributes of an handler and if it
  does, it will consider two elements to be the same and override one another
  if they have the same url and apply the same strategy to serve files.

  This is important as - for a given url - there can only be a single
  handler, so the configurations need to be merged.
  """
  if not isinstance(el, dict) or not "url" in el:
    try:
      return dest.index(el)
    except ValueError:
      return -1

  for ix, existing in enumerate(dest):
    # See docstring for explanation. Tl;Dr: one url config overrides
    # another if the url is the same, and if both configs are static_files
    # or both configs are static_dir.
    # A static_dir and a static_files entry can coexist for the same
    # even if they refer to the same url.
    if ("url" in existing and existing["url"] == el["url"] and
        (("static_dir" in existing and "static_dir" in el) or
         ("static_files" in existing and "static_files" in el))):
      return ix

  return -1

def merge(dest: dict, source: dict) -> dict:
  """Simple recurisve merge of two dicts with some app.yaml awareness."""
  merged = {}

  tomerge = list(dest.items()) + list(source.items())
  for key, value in tomerge:
    if key not in merged:
      merged[key] = copy.deepcopy(value)
      continue

    if isinstance(value, list):
      target = merged[key]
      if not isinstance(target, list):
        merged[key] = value
        continue

      for el in value:
        ix = handler_ix(target, el)
        if ix < 0:
          target.append(el)
        else:
          target[ix] = merge(target[ix], el)
      continue

    if isinstance(value, dict):
      merged[key] = merge(merged[key], value)
      continue

    merged[key] = copy.deepcopy(value)
  return merged

def main(argv):
  parser = argparse.ArgumentParser()
  parser.add_argument("-l", "--login", default="",
      help="If specifiled, a 'login: <value-specified>' will be added to heach handler")
  parser.add_argument("-x", "--extra", default=[], action="append",
      help="If specifiled, each string supplied will be added as a parameter to all handlers")
  parser.add_argument("-e", "--header", default=[], action="append",
      help="If specifiled, each string supplied will be added as a header to all handlers")
  parser.add_argument("input", help="One or more yaml file to process", nargs="+")
  args = parser.parse_args(argv[1:])

  config = {}
  for inpath in args.input:
    with open(inpath, "r") as infile:
      inyaml = yaml.safe_load(infile)
      config = merge(config, inyaml)

  if args.header or args.login and "handlers" in config: 
    for handler in config["handlers"]:
      if args.login:
        handler["login"] = args.login

      if args.header:
        headers = handler.setdefault("http_headers", {})
        for header in args.header:
          key, value = header.split(":", 1)
          headers[key] = value.strip()

      if args.extra:
        for extra in args.extra:
          key, value = extra.split(":", 1)
          handler[key] = value.strip()

  yaml.dump(config, sys.stdout, sort_keys=False)
  return 0

if __name__ == "__main__":
  sys.exit(main(sys.argv))
