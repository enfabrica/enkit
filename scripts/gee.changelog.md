# gee changelog

## Releases

### Unreleased

* #594: `gee rmbr` now also removes bazel cache directory.

### Release 0.2.29

* Streamling PR submission, and make the PR description template more terse.
* Fix a malformed `git diff` command used during PR description template creation.
* #565: Disable the unreliable diff test after `gee pr_submit`.
* #576: `gee pr_submit` now checks PR status before attemping to submit.  gee
  will no longer attempt to submit a PR that has already been merged.
* #561: disable --autostash on all `git rebase` operations.  The user must
  commit all changes before updated.
* #560: `gee version` now checks if a newer version of gee is available.
* #551: fix `gee cleanup`'s handling of `pr_NNN` branches created using `gee pr_checkout`.

### Release 0.2.28

* #550: Fix formatting of date when creating a shallow clone of the git repo.
* Fix `gee cleanup`: fix diff'ing of `pr_checkout` branches

### Release 0.2.27

* Add `pr_rollback` command.

### Release 0.2.26

* Fixed exception in `pr_make` due to empty read error.

### Release 0.2.25

* Added `pr_push` command.

### Release 0.2.24

* Added `restore_all_branches` command.

### Release 0.2.23

* Rendered gee help as a user manual, in markdown.
* `pr_make`: Add support for marking PRs as "DRAFTs" or "ABORTing".
