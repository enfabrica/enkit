#!/usr/bin/env bash

set -e

exec {wrapper} \
  {upload_file_flags} \
  {astore_path_flag} \
  {uidfile_flag} \
  {tag_flags} \
  {output_format_flag}
