.PHONY: update-go-deps
update-go-deps:
	bazel run @rules_go//go -- mod tidy -v
	bazel run //:gazelle_update_repos
	bazel mod tidy
