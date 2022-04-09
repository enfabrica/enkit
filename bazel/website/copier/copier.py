"""Script to copy a subset of files to fixed destinations based on patterns.

This script is not meant to be used directly. Check the website_packaged_tree
rule instead.
"""
import argparse
import json
import fnmatch
import os
import shlex
import sys

def copy(params, origin, froot, files, matches):
  for input in files:
    for match in matches:
      if not fnmatch.fnmatch(input, match.get("match", "*")):
        continue

      output = input
      for prefix in match.get("strip", []):
        if input.startswith(prefix):
          output = input[len(prefix):]
          break

      dest = os.path.join(match["dest"].strip("/"), output.strip("/"))
      fulldest = os.path.join(params.webroot, dest)
      fullinput = os.path.join(froot, input)

      if "copy" in params.show:
        print(f"{params.package}, {origin} - copy: {input} {dest}")
        print(f"  match:{match.get('match', '*')} strip:{match.get('strip', [])} from:{froot} to:{params.webroot}")

      command = "mkdir -p {dir}; cp -fLpr {input} {dest}".format(
          dir=shlex.quote(os.path.dirname(fulldest)),
          input=shlex.quote(fullinput),
          dest=shlex.quote(fulldest)
      )
      os.system(command)

def printtree(params, directory, name):
  print(f"{params.package}: location {directory}")
  print(f"================ START - {params.package} {name} TREE - {directory}")
  tree = os.popen(f"cd {shlex.quote(directory)}; find -L -printf '%P\n'").read().strip()
  print(f"{tree}")
  print(f"================ END - {params.package} {name} TREE")

def main(argv):
  parser = argparse.ArgumentParser()
  parser.add_argument("-p", "--package", default="<unknown>", help="Bazel package on behalf of which this copy is performed")
  parser.add_argument("-w", "--webroot", help="Destination directory where to copy all the files", required=True)
  parser.add_argument("-c", "--config", help="Config defining the files to copy", required=True)
  parser.add_argument("-s", "--show", default=[], action="append", help="Debug messages to show on screen (one or more of 'config', 'source', 'dest' or 'copy')")
  params = parser.parse_args(argv[1:])

  config = open(params.config).read()
  if "config" in params.show:
    print(f"{params.package} using config file: {params.config}\nconfig:\n{config}")
  
  if "source" in params.show:
    printtree(params, os.getcwd(), "INPUT")
  
  for target in json.loads(config):
    for input in target["files"]:
      if input["trel"].endswith("/"):
        command = os.popen("cd {dir}; find -L -printf '%P\n'".format(dir=shlex.quote(input["tpath"])))
        copy(params, target["package"], input["tpath"], command.read().strip().split("\n"), target["matches"])
      else:
        copy(params, target["package"], input["troot"], [input["trel"]], target["matches"])
  
  if "dest" in params.show:
    printtree(params, params.webroot, "OUTPUT")

if __name__ == "__main__":
  sys.exit(main(sys.argv))
