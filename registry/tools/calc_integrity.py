#!/usr/bin/env python3
#
# Copyright 2022 The Bazel Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import sys
import hashlib
import base64

def integrity(data, algorithm="sha256"):
    assert algorithm in {
        "sha224",
        "sha256",
        "sha384",
        "sha512",
    }, "Unsupported SRI algorithm"
    hash = getattr(hashlib, algorithm)(data)
    encoded = base64.b64encode(hash.digest()).decode()
    return f"{algorithm}-{encoded}"

def read(path):
    with open(path, "rb") as file:
        return file.read()

if __name__ == "__main__":
    print(integrity(read(sys.argv[1])))
