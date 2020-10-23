#!/bin/bash
echo "START WORRYING"
for arg in "$@"; do
  echo "  ARG $arg"
done

echo "=== ENV ==="
printenv |grep KCONFIG
echo "=== END ==="
