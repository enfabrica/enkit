#!/bin/bash

cd "$0".runfiles/enkit/
exec "./bazel/typescript/protoc-gen-ts-proto.sh"
