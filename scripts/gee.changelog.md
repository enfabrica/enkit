# gee changelog

## Releases

### 0.2.37

* Re-enable `--autostash` behavior when rebasing (#780)

### 0.2.36

* Timeout ssh-add -l if the ssh-agent gets stuck (#768)
* `gee pr_checks`: make output quieter, add --wait flag (#749)
* `gee restore_all_branches` works from non-gee dirs (#743)

### 0.2.35

* `gee pr_checks`: report the specific failing test that caused a presubmit check to fail. (#739)
* `gee config`: turn on rerere.enabled for all users. (#738)
* `gee pr_rerun`: re-trigger PR presubmit tests. (#737)

### 0.2.34

* `gee`: always specify full email address when authenticating to gh (#731)
* `gee init`: use the new gh auth flow more consistently (#732)
* `gee`: do our best regardless of which directory gee is invoked from (#699)
* `gee gcd`: fix bug where savelog output would break gcd (#728)
* `gee lsbr`: fix bug where upstream branches would break lsbr (#710)
* `gee restore_all_branches`: interactively pick which branches to restore (#682)
* `gee init`: improve `gh auth login` flow, use right options instead of asking (#676)

### 0.2.33

* `gee update`: improve merge conflict resolution flow (#715)
* `gee repair`: auto-fix misconfigured gh-resolved attribute. (#719)
* `gcd`: fix bug in parsing of `git worktree list --porcelain` output (#713)
* `gee bash_setup`: fix labeling on non-printing prompt characters (#696)
* `gee repair`: perform `git worktree prune` as a repair step (#678)

### 0.2.32

* `gee gcd`: enable `gcd -m` to quickly create a branch of master (#645)
* `gee rmbr`: remove multiple branches at a go. (#645)
* `gee.md`: improve documentation around rebase operations (#664)
* `gee hello`: check for ssh keyfile conditions (#672)
* `gee bazelgc`: handle no dirs to delete case (#663)
* `gee diagnose`: add ssh diagnostics (#636)

### 0.2.31

* `gee rmbr`: fails functional if the directory bazel-out links to has already
  been removed. (#630)
* `gee pr_make`: Make parsing of PR description more robust by parsing comments
  like blank lines. (#629)
* `gee diff`: show only new changes in this branch since branches diverged (#628)
* `gee lspr`: fix incomplete reviews list (#627)
*
### 0.2.30

* #614: Added logging and `gee diagnose` commands.
* #596: Added `gee bazelgc` command removes old bazel cache directories.
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
