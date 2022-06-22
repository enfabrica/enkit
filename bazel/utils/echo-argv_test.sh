#!/bin/bash

# Just outputs each argv parameter, so another script can
# verify that all parameters were passed correctly.
for arg in "$@"; do
  echo "$arg"
done
