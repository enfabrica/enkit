#!/usr/bin/env bash

exec {wrapper} \
  {upload_file_flags} \
  {astore_path_flag} \
  {tag_flags} \
  {output_format_flag}
