"""A tool to scan a directory tree and generate snippets of an app.yaml file.

When configuring static paths in app.yaml file, there are a few things that may
need to be done manually. Specifically, if there's a desire to have an url like
'directory/' to serve the file 'directory/index.html', there has to be a static
mapping.

At times, it is also necessary to provide mime types explicitly.

This script scans a directory tree, and based on command line flags, expands a
per file template providing inputs.
"""
import os
import sys
import argparse
import mimetypes
import textwrap

def main(argv):
  parser = argparse.ArgumentParser()
  parser.add_argument("-p", "--prefix", default="handlers:", help="Optional prefix to prepend to the output")
  parser.add_argument("-i", "--index", action="append", help="Index files to look for - if not specified, will process all files")
  parser.add_argument("-t", "--template", help="Path to a template file to use to generate each entry in the output")
  parser.add_argument("-u", "--url-strip", default=[], action="append", help="Prefixes to strip from the path to create an url")
  parser.add_argument("-r", "--root-strip", default="", help="Prefixes to strip from the path to create the path of the file in the config")
  parser.add_argument("-l", "--login", default="", help="If specifiled, a 'login: <value-specified>' will be added to the generated handlers")
  parser.add_argument("dir", help="One or more directories to scan for files", nargs="+")
  args = parser.parse_args(argv[1:])

  template = textwrap.dedent("""\
      url: {urldir}
      static_files: {filename}
      upload: {filename}""")
  if args.template:
    template = open(args.template()).read().strip()

  if args.prefix:
    print(args.prefix)

  config = {}
  for indir in args.dir:
    for root, subdirs, subfiles in os.walk(indir):
      if args.index:
        for index in args.index:
          if index in subfiles:
            subfiles = [index]
            break
        else:
          continue

      for index in subfiles:
        filename = os.path.join(root, index)
        mimetype, mimeencoding = mimetypes.guess_type(filename)
        if mimeencoding is None:
          mimeencoding = ""

        urlfile = filename
        urldir = os.path.dirname(urlfile)
        for strip in args.url_strip:
          if urlfile.startswith(strip):
            urlfile = urlfile[len(strip):]
            urldir = os.path.dirname(urlfile)
            break

        # Why? Let's say /dir is mapped to  /dir/index.html.
        # Any relative path in index.html will not work, will use / as parent directory.
        # Instead, we need to map /dir/ to /dir/index.thml.
        urldir = os.path.join(urldir, "")

        if filename.startswith(args.root_strip):
          filename = filename[len(args.root_strip):]
        filename = filename.strip("/")
        filedir = os.path.dirname(filename)

        expanded = template.format(
           filename = filename,
           filedir = filedir,
           urldir = urldir,
           urlfile = urlfile,
           mimetype = mimetype,
           mimeencoding = mimeencoding
        ).split("\n")

        if args.login:
          expanded.append(f"login: {args.login}")

        wrapped = "- " + "\n".join(["  " + l for l in expanded])[2:]
        print(wrapped)
    
if __name__ == "__main__":
  sys.exit(main(sys.argv))
