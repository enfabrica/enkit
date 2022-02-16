load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@io_bazel_rules_go//extras:embed_data_deps.bzl", "go_embed_data_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")
load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies", "rules_proto_toolchains")
load("@build_bazel_rules_nodejs//:index.bzl", "yarn_install")
load("@rules_pkg//:deps.bzl", "rules_pkg_dependencies")
load("@com_github_atlassian_bazel_tools//multirun:deps.bzl", "multirun_dependencies")
load("//bazel/ui:deps.bzl", "install_ui_deps")
load("//bazel/linting:deps.bzl", "lint_deps_init")

def enkit_init_go():
    go_rules_dependencies()
    go_register_toolchains(version = "1.16")
    go_embed_data_dependencies()
    gazelle_dependencies()

def enkit_init_proto():
    rules_proto_dependencies()
    rules_proto_toolchains()

def enkit_init_ts():
    yarn_install(
        name = "npm",
        package_json = "//ui:package.json",
        yarn_lock = "//ui:yarn.lock",
    )

def enkit_init_tools():
    rules_pkg_dependencies()
    multirun_dependencies()

def enkit_init():
    enkit_init_go()
    enkit_init_proto()
    enkit_init_ts()
    enkit_init_tools()
    install_ui_deps()
    lint_deps_init()
