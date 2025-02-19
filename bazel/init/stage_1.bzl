"""Stage 1 configuration for enkit WORKSPACE.

See README.md for more information.
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")
load("@bazel_tools//tools/build_defs/repo:utils.bzl", "maybe")

def stage_1():
    """Stage 1 initialization for WORKSPACE.

    This step includes mostly direct dependencies. As long as this function has
    no repository rule/macro calls, and is invoked first in the WORKSPACE file,
    we can reasonably assume that each repository listed below is the version
    specified in this file, regardless of the order in which they are declared.
    Because the first WORKSPACE entry wins, any repository added to this list
    will override dependendencies loaded as part of later stages, which can be a
    way of forcing a dependency upgrade underneath e.g. io_bazel_rules_go.
    """
    maybe(
        name = "aspect_bazel_lib",
        repo_rule = http_archive,
        sha256 = "688354ee6beeba7194243d73eb0992b9a12e8edeeeec5b6544f4b531a3112237",
        strip_prefix = "bazel-lib-2.8.1",
        url = "https://github.com/aspect-build/bazel-lib/releases/download/v2.8.1/bazel-lib-v2.8.1.tar.gz",
    )

    maybe(
        name = "rules_distroless",
        repo_rule = http_archive,
        sha256 = "8a3440067453ad211f3b34d4a8f68f65663dc5fd6d7834bf81eecf0526785381",
        strip_prefix = "rules_distroless-0.3.6",
        url = "https://github.com/GoogleContainerTools/rules_distroless/releases/download/v0.3.6/rules_distroless-v0.3.6.tar.gz",
    )

    maybe(
        name = "io_bazel_rules_go",
        repo_rule = http_archive,
        patches = ["@enkit//bazel/dependencies/io_bazel_rules_go:tags_manual.patch"],
        sha256 = "278b7ff5a826f3dc10f04feaf0b70d48b68748ccd512d7f98bf442077f043fe3",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.41.0/rules_go-v0.41.0.zip",
            "https://github.com/bazelbuild/rules_go/releases/download/v0.41.0/rules_go-v0.41.0.zip",
        ],
    )

    maybe(
        name = "bazel_gazelle",
        repo_rule = http_archive,
        patches = [
            "@enkit//bazel/dependencies/bazel_gazelle:dont_flatten_srcs.patch",
        ],
        patch_args = ["-p1"],
        sha256 = "b7387f72efb59f876e4daae42f1d3912d0d45563eac7cb23d1de0b094ab588cf",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.34.0/bazel-gazelle-v0.34.0.tar.gz",
            "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.34.0/bazel-gazelle-v0.34.0.tar.gz",
        ],
    )

    maybe(
        name = "rules_proto",
        repo_rule = http_archive,
        sha256 = "e017528fd1c91c5a33f15493e3a398181a9e821a804eb7ff5acdd1d2d6c2b18d",
        strip_prefix = "rules_proto-4.0.0-3.20.0",
        urls = [
            "https://github.com/bazelbuild/rules_proto/archive/refs/tags/4.0.0-3.20.0.tar.gz",
        ],
    )

    maybe(
        name = "bats_support",
        repo_rule = http_archive,
        url = "https://github.com/bats-core/bats-support/archive/refs/tags/v0.3.0.tar.gz",
        strip_prefix = "bats-support-0.3.0",
        build_file = "@enkit//bazel/dependencies:BUILD.bats_support.bazel",
        sha256 = "7815237aafeb42ddcc1b8c698fc5808026d33317d8701d5ec2396e9634e2918f",
    )

    maybe(
        name = "bats_assert",
        repo_rule = http_archive,
        url = "https://github.com/bats-core/bats-assert/archive/refs/tags/v2.0.0.tar.gz",
        strip_prefix = "bats-assert-2.0.0",
        build_file = "@enkit//bazel/dependencies:BUILD.bats_assert.bazel",
        sha256 = "15dbf1abb98db785323b9327c86ee2b3114541fe5aa150c410a1632ec06d9903",
    )

    maybe(
        name = "bats_file",
        repo_rule = http_archive,
        url = "https://github.com/bats-core/bats-file/archive/refs/tags/v0.2.0.tar.gz",
        strip_prefix = "bats-file-0.2.0",
        build_file = "@enkit//bazel/dependencies:BUILD.bats_file.bazel",
        sha256 = "1fa26407a68f4517cf9150d4763779ee66946a68eded33fa182ddf6a795c5062",
    )

    maybe(
        name = "bats_core",
        repo_rule = http_archive,
        url = "https://github.com/bats-core/bats-core/archive/refs/tags/v1.5.0.tar.gz",
        strip_prefix = "bats-core-1.5.0",
        sha256 = "36a3fd4413899c0763158ae194329af8f48bb1ff0d1338090b80b3416d5793af",
        patch_args = ["-p1"],
        patch_tool = "patch",
        patches = [
            "@enkit//bazel/dependencies:bats_root.patch",
        ],
        build_file = "@enkit//bazel/dependencies:BUILD.bats.bazel",
    )

    maybe(
        name = "rules_pkg",
        repo_rule = http_archive,
        urls = [
            "https://github.com/bazelbuild/rules_pkg/releases/download/0.9.1/rules_pkg-0.9.1.tar.gz",
            "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.9.1/rules_pkg-0.9.1.tar.gz",
        ],
        sha256 = "8f9ee2dc10c1ae514ee599a8b42ed99fa262b757058f65ad3c384289ff70c4b8",
    )

    maybe(
        name = "com_github_atlassian_bazel_tools",
        repo_rule = http_archive,
        strip_prefix = "bazel-tools-5c3b9306e703c6669a6ce064dd6dde69f69cba35",
        sha256 = "c8630527150f3a9594e557fdcf02694e73420c10811eb214b461e84cb74c3aa8",
        urls = [
            "https://github.com/atlassian/bazel-tools/archive/5c3b9306e703c6669a6ce064dd6dde69f69cba35.zip",
        ],
    )

    maybe(
        name = "com_google_absl",
        repo_rule = http_archive,
        sha256 = "7c11539617af1f332f0854a6fb21e296a1b29c27d03f23c7b49d4adefcd102cc",
        strip_prefix = "abseil-cpp-20230802.2",
        patch_args = ["-p1"],
        patches = [
            "@enkit//bazel/dependencies/abseil:0001-remove-maes-and-msse4.1-option-from-cross-compilation.patch",
        ],
        urls = ["https://github.com/abseil/abseil-cpp/archive/refs/tags/20230802.2.tar.gz"],
    )

    maybe(
        name = "bazel_skylib",
        repo_rule = http_archive,
        sha256 = "bc283cdfcd526a52c3201279cda4bc298652efa898b10b4db0837dc51652756f",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.7.1/bazel-skylib-1.7.1.tar.gz",
            "https://github.com/bazelbuild/bazel-skylib/releases/download/1.7.1/bazel-skylib-1.7.1.tar.gz",
        ],
    )

    maybe(
        name = "rules_python",
        repo_rule = http_archive,
        strip_prefix = "rules_python-0.32.0",
        urls = [
            "https://github.com/bazelbuild/rules_python/releases/download/0.32.0/rules_python-0.32.0.tar.gz",
        ],
    )

    maybe(
        name = "boringssl",
        repo_rule = http_archive,
        patch_args = ["-p1"],
        patches = [
            "@enkit//bazel/dependencies/boringssl:0001-move-hrss-polynomial-declarations-under-x64-flag.patch",
            "@enkit//bazel/dependencies/boringssl:0002-commentout-fips-module-AARCH64-declarations.patch",
        ],
        sha256 = "534fa658bd845fd974b50b10f444d392dfd0d93768c4a51b61263fd37d851c40",
        strip_prefix = "boringssl-b9232f9e27e5668bc0414879dcdedb2a59ea75f2",
        urls = [
            "https://storage.googleapis.com/grpc-bazel-mirror/github.com/google/boringssl/archive/b9232f9e27e5668bc0414879dcdedb2a59ea75f2.tar.gz",
            "https://github.com/google/boringssl/archive/b9232f9e27e5668bc0414879dcdedb2a59ea75f2.tar.gz",
        ],
    )

    maybe(
        name = "com_github_grpc_grpc",
        repo_rule = http_archive,
        patch_args = ["-p1"],
        patches = [
            "@enkit//bazel/dependencies/grpc:no_remote_tag.patch",
            "@enkit//bazel/dependencies/grpc:use_hermetic_py_headers.patch",
        ],
        sha256 = "e18b16f7976aab9a36c14c38180f042bb0fd196b75c9fd6a20a2b5f934876ad6",
        strip_prefix = "grpc-1.45.2",
        urls = [
            "https://github.com/grpc/grpc/archive/refs/tags/v1.45.2.tar.gz",
        ],
    )

    maybe(
        name = "com_github_bazelbuild_buildtools",
        repo_rule = http_archive,
        strip_prefix = "buildtools-5.1.0",
        url = "https://github.com/bazelbuild/buildtools/archive/refs/tags/5.1.0.tar.gz",
        sha256 = "e3bb0dc8b0274ea1aca75f1f8c0c835adbe589708ea89bf698069d0790701ea3",
    )

    maybe(
        name = "rules_oci",
        repo_rule = http_archive,
        strip_prefix = "rules_oci-1.7.5",
        url = "https://github.com/bazel-contrib/rules_oci/releases/download/v1.7.5/rules_oci-v1.7.5.tar.gz",
        sha256 = "56d5499025d67a6b86b2e6ebae5232c72104ae682b5a21287770bd3bf0661abf",
    )

    maybe(
        name = "io_bazel_rules_docker",
        repo_rule = http_archive,
        sha256 = "27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820",
        strip_prefix = "rules_docker-0.24.0",
        urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.24.0/rules_docker-v0.24.0.tar.gz"],
        patches = [
            "@enkit//bazel/dependencies:rules_docker_no_init_go.patch",
        ],
        patch_args = ["-p1"],
    )

    maybe(
        name = "com_google_googleapis",
        repo_rule = http_archive,
        urls = ["https://github.com/googleapis/googleapis/archive/f5ed6db308e6ce3f9bcdc3afcbf2ab8b50d905d6.zip"],
        strip_prefix = "googleapis-f5ed6db308e6ce3f9bcdc3afcbf2ab8b50d905d6",
        sha256 = "f8f615f7c21459cb9b6ec2efaf795c875cd4698d6a1814a0a30d1eb910903142",
    )

    maybe(
        name = "com_google_protobuf",
        repo_rule = http_archive,
        sha256 = "8b28fdd45bab62d15db232ec404248901842e5340299a57765e48abe8a80d930",
        strip_prefix = "protobuf-3.20.1",
        urls = ["https://github.com/protocolbuffers/protobuf/archive/refs/tags/v3.20.1.tar.gz"],
    )

    # BUG(INFRA-6710): `make` is pulled in by source by rules_foreign_cc, but we
    # need to patch its configure script to not care so much about file
    # timestamps, as buildbarn's FUSE workers may not expose file timestamps as
    # expected.
    maybe(
        name = "gnumake_src",
        repo_rule = http_archive,
        sha256 = "581f4d4e872da74b3941c874215898a7d35802f03732bdccee1d4a7979105d18",
        strip_prefix = "make-4.4",
        build_file_content = """
filegroup(
    name = "all_srcs",
    srcs = glob(["**"]),
    visibility = ["//visibility:public"],
)
""",
        urls = [
            "https://mirror.bazel.build/ftpmirror.gnu.org/gnu/make/make-4.4.tar.gz",
            "http://ftpmirror.gnu.org/gnu/make/make-4.4.tar.gz",
        ],
        patches = [
            "@enkit//bazel/dependencies:make_less_pedantic_configure.patch",
        ],
        patch_args = ["-p1"],
    )

    maybe(
        name = "rules_foreign_cc",
        repo_rule = http_archive,
        sha256 = "9561b3994232ccb033278ade83c2ce48e763e9cae63452cd8fea457bedd87d05",
        strip_prefix = "rules_foreign_cc-816905a078773405803e86635def78b61d2f782d",
        url = "https://github.com/bazelbuild/rules_foreign_cc/archive/816905a078773405803e86635def78b61d2f782d.tar.gz",
        patches = [
            "@enkit//bazel/dependencies:rules_foreign_cc_export_functions.patch",
            "@enkit//bazel/dependencies:rules_foreign_cc_module_linker_flags.patch",
        ],
        patch_args = ["-p1"],
    )

    maybe(
        name = "meson",
        repo_rule = http_archive,
        build_file = "@enkit//bazel/meson:meson.BUILD.bazel",
        sha256 = "d04b541f97ca439fb82fab7d0d480988be4bd4e62563a5ca35fadb5400727b1c",
        urls = ["https://github.com/mesonbuild/meson/releases/download/1.1.1/meson-1.1.1.tar.gz"],
        strip_prefix = "meson-1.1.1",
    )

    maybe(
        name = "net_enfabrica_binary_astore",
        repo_rule = http_file,
        sha256 = "47be8fa1067a8c498a67888b6f32386b9504b70e1da13afe869e6f06139805c9",
        urls = ["https://astore.corp.enfabrica.net/d/bazel/workspace_deps/astore/v1?u=7hfw4dsxxobamx5uyvvwmnj8tpxj7yub"],
        executable = True,
    )

    maybe(
        name = "libz",
        repo_rule = http_archive,
        build_file = "@enkit//bazel/dependencies:libz.BUILD.bazel",
        sha256 = "c3e5e9fdd5004dcb542feda5ee4f0ff0744628baf8ed2dd5d66f8ca1197cb1a1",
        strip_prefix = "zlib-1.2.11",
        # Original file: https://zlib.net/fossils/zlib-1.2.11.tar.gz
        urls = ["https://astore.corp.enfabrica.net/d/mirror/zlib/zlib-1.2.11.tar.gz?u=giqzp6y6me76syf7jrgwtevqxgdhswdu"]
    )

    maybe(
        name = "dropbear",
        repo_rule = http_archive,
        build_file = "@enkit//bazel/dependencies:dropbear.BUILD.bazel",
        sha256 = "d16285f0233a2400a84affa0235e34a71c660908079c639fdef889c2e90c9f5f",
        urls = ["https://github.com/mkj/dropbear/archive/refs/tags/DROPBEAR_2024.86.tar.gz"],
        strip_prefix = "dropbear-DROPBEAR_2024.86",
        patches = [
            "@enkit//bazel/dependencies/dropbear:0001-allow-blank-password.patch",
            "@enkit//bazel/dependencies/dropbear:0002-override-authorized-keys.patch",
            "@enkit//bazel/dependencies/dropbear:0003-ignore-user-s-shell.patch",
        ],
        patch_args = ["-p1"],
    )

    maybe(
        name = "rules_proto_grpc",
        repo_rule = http_archive,
        sha256 = "2a0860a336ae836b54671cbbe0710eec17c64ef70c4c5a88ccfd47ea6e3739bd",
        urls = ["https://github.com/rules-proto-grpc/rules_proto_grpc/releases/download/4.6.0/rules_proto_grpc-4.6.0.tar.gz"],
        strip_prefix = "rules_proto_grpc-4.6.0",
    )

    # Explicitly load Jsonnet here so that we control the version, instead of
    # rules_jsonnet and dependencies, which tend to use an old version.
    maybe(
        name = "cpp_jsonnet",
        repo_rule = http_archive,
        sha256 = "77bd269073807731f6b11ff8d7c03e9065aafb8e4d038935deb388325e52511b",
        strip_prefix = "jsonnet-0.20.0",
        urls = [
            "https://github.com/google/jsonnet/archive/v0.20.0.tar.gz",
        ],
    )

    # Required by buildbarn ecosystem
    http_archive(
        name = "com_github_bazelbuild_buildtools",
        sha256 = "42968f9134ba2c75c03bb271bd7bb062afb7da449f9b913c96e5be4ce890030a",
        strip_prefix = "buildtools-6.3.3",
        url = "https://github.com/bazelbuild/buildtools/archive/v6.3.3.tar.gz",
    )

    # Required by buildbarn ecosystem
    maybe(
        name = "googleapis",
        repo_rule = http_archive,
        urls = ["https://github.com/googleapis/googleapis/archive/f5ed6db308e6ce3f9bcdc3afcbf2ab8b50d905d6.zip"],
        strip_prefix = "googleapis-f5ed6db308e6ce3f9bcdc3afcbf2ab8b50d905d6",
        sha256 = "f8f615f7c21459cb9b6ec2efaf795c875cd4698d6a1814a0a30d1eb910903142",
    )

    # Required by buildbarn ecosystem
    maybe(
        name = "io_bazel_rules_jsonnet",
        repo_rule = http_archive,
        sha256 = "d20270872ba8d4c108edecc9581e2bb7f320afab71f8caa2f6394b5202e8a2c3",
        strip_prefix = "rules_jsonnet-0.4.0",
        urls = ["https://github.com/bazelbuild/rules_jsonnet/archive/0.4.0.tar.gz"],
    )

    # Required by buildbarn ecosystem
    maybe(
        name = "com_github_twbs_bootstrap",
        repo_rule = http_archive,
        build_file_content = """exports_files(["css/bootstrap.min.css", "js/bootstrap.min.js"])""",
        sha256 = "395342b2974e3350560e65752d36aab6573652b11cc6cb5ef79a2e5e83ad64b1",
        strip_prefix = "bootstrap-5.1.0-dist",
        urls = ["https://github.com/twbs/bootstrap/releases/download/v5.1.0/bootstrap-5.1.0-dist.zip"],
    )

    # Required by buildbarn ecosystem
    maybe(
        name = "aspect_rules_js",
        repo_rule = http_archive,
        sha256 = "00e7b97b696af63812df0ca9e9dbd18579f3edd3ab9a56f227238b8405e4051c",
        strip_prefix = "rules_js-1.23.0",
        url = "https://github.com/aspect-build/rules_js/releases/download/v1.23.0/rules_js-v1.23.0.tar.gz",
    )

    # Required by buildbarn ecosystem
    http_archive(
        name = "rules_antlr",
        patches = ["@enkit//bazel/dependencies/rules_antlr:antlr_4.10.patch"],
        sha256 = "26e6a83c665cf6c1093b628b3a749071322f0f70305d12ede30909695ed85591",
        strip_prefix = "rules_antlr-0.5.0",
        urls = ["https://github.com/marcohu/rules_antlr/archive/0.5.0.tar.gz"],
    )

    # Required by buildbarn ecosystem
    maybe(
        name = "io_opentelemetry_proto",
        repo_rule = http_archive,
        build_file_content = """
proto_library(
    name = "common_proto",
    srcs = ["opentelemetry/proto/common/v1/common.proto"],
    visibility = ["//visibility:public"],
)
""",
        sha256 = "464bc2b348e674a1a03142e403cbccb01be8655b6de0f8bfe733ea31fcd421be",
        strip_prefix = "opentelemetry-proto-0.19.0",
        urls = ["https://github.com/open-telemetry/opentelemetry-proto/archive/refs/tags/v0.19.0.tar.gz"],
    )
