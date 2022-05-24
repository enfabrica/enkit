filegroup(name = "{name}", srcs = glob(["lib/modules/**"], exclude = ["**/build/**", "**/source/**"]), visibility = ["//visibility:public"])
