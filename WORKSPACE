# gazelle:repository_macro bazel/go_repositories.bzl%go_repositories

workspace(
    name = "enkit",
)

load("//bazel/init:stage_1.bzl", "stage_1")

stage_1()

load("//bazel:go_repositories.bzl", "go_repositories")

# This call is placed here intentionally outside of the steps so that enkit and
# any downstream repos can keep seperate go dependencies.
go_repositories()

load("//bazel/init:stage_2.bzl", "stage_2")

stage_2()

load("//bazel/init:stage_3.bzl", "stage_3")

stage_3()

load("//bazel/init:stage_4.bzl", "stage_4")

stage_4()

# BASIC rules_js

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "aspect_rules_js",
    sha256 = "dda5fee3926e62c483660b35b25d1577d23f88f11a2775e3555b57289f4edb12",
    strip_prefix = "rules_js-1.6.9",
    url = "https://github.com/aspect-build/rules_js/archive/refs/tags/v1.6.9.tar.gz",
)

load("@aspect_rules_js//js:repositories.bzl", "rules_js_dependencies")

rules_js_dependencies()

load("@rules_nodejs//nodejs:repositories.bzl", "DEFAULT_NODE_VERSION", "nodejs_register_toolchains")

nodejs_register_toolchains(
    name = "nodejs",
    node_version = DEFAULT_NODE_VERSION,
)

load("@aspect_rules_js//npm:npm_import.bzl", "npm_translate_lock")

npm_translate_lock(
    name = "npm",
    pnpm_lock = "//:pnpm-lock.yaml",
    verify_node_modules_ignored = "//:.bazelignore",
)

load("@npm//:repositories.bzl", "npm_repositories")

npm_repositories()

# rules_ts

http_archive(
    name = "aspect_rules_ts",
    sha256 = "f3f0d0a92b0069f8d1bf6a0e26408bd591a8626166db3f88e8d971ffed8f59ba",
    strip_prefix = "rules_ts-1.0.0",
    url = "https://github.com/aspect-build/rules_ts/archive/refs/tags/v1.0.0.tar.gz",
)

load("@aspect_rules_ts//ts:repositories.bzl", "LATEST_VERSION", "rules_ts_dependencies")

rules_ts_dependencies(ts_version = LATEST_VERSION)

# rules_esbuild

http_archive(
    name = "aspect_rules_esbuild",
    sha256 = "dccab34d457faf9968ec83e2900d65cf5b846f036822b675d988deee0113dba9",
    strip_prefix = "rules_esbuild-0.13.1",
    url = "https://github.com/aspect-build/rules_esbuild/archive/refs/tags/v0.13.1.tar.gz",
)

load("@aspect_rules_esbuild//esbuild:dependencies.bzl", "rules_esbuild_dependencies")

rules_esbuild_dependencies()

# Register a toolchain containing esbuild npm package and native bindings
load("@aspect_rules_esbuild//esbuild:repositories.bzl", "LATEST_VERSION", "esbuild_register_toolchains")

esbuild_register_toolchains(
    name = "esbuild",
    esbuild_version = LATEST_VERSION,
)
