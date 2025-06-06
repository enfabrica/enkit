# inspired by https://github.com/bazel-contrib/rules_python/issues/1679#issuecomment-2249536549

import os

__path__.append(os.path.abspath(__path__[0] + "/.."))
