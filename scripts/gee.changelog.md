# gee changelog

## Releases

### 0.2.56

* `gee lsbr`: fix bug in traversing upstream branches.

### 0.2.55

* `gee hello`: diagnose missing github API token scopes.
* `gee update`: fix fetching of commits from other user's PRs.

### 0.2.54

* `gee pr_checkout`: add "-n" flag to explicitly specify a branch name.
* `gee rmbr`: fix bug when deleting branches created by `gee pr_checkout`.

### 0.2.53
* `gee pick`: add cherry-picking command with metadata logging.  (#1169, #1171)
* `gee pr_make`: add AUTOMERGE option. (#1172)
* `gee mkbr`: fix and simplify behavior when branch exists in origin. (#1177)

### 0.2.52
* `gee lspr`: Improve lspr output, improve bash prompt to report number of assigned PRs.  (#1148)
* `gee pr_submit`: Fix automatic rebasing of child branches.  This broke in 0.2.51.  (#1147)
* `gee up`: Fix bug introduced in 0.2.51: Correctly pulls new commits from origin.  (#1146)
* `gee whatsout`: Fix bug if running in a branch created by `gee pr_checkout`.  (#1145)
* `gee gcd`: Fail correctly if target branch isn't specified.  (#1144)
* `gee vimdiff`: Now uses `git difftool` (#1125)

### 0.2.51
* `gee pr_make`: Darn, fixed a typo that I missed before.  (#1118)
* `gee codeowners`: Fix codeowners to work with non-standard base branches.  (#1116)
* `gee pr_make`: Allow branches to be created from arbitrary upstream base branches. (#1088)
* `gee config`: Update to bcompare5.  (#1102)
* `gee copy`: Added "copy" command to facilitate copying files while preserving history. (#1093)
* `gee rmbr`: When removing multiple branches, remove remaining branches even if one branch removal fails. (#1087)
* `gee`: Allow explicit paths for enkit (#358)
* `gee rupdate`: Exit recursion when a branch has itself as a parent (#1075)
* `gee commit`: Improve output when cancelling running gcloud jobs.  #(1068)

### 0.2.50

* gee commit: cancels invalidates presubmits, on by default. (#1067)
* gee find: only search the bazel symlinks if primary search fails.  (#1064)
* gee bisect: add `--good` option. (#1057)
* gee pr_make: don't auto-assign @me if PR is a draft. (#1045)

### 0.2.49

* gee hello: also check and repair gh authentication (#1042).
* gee up: fix resolution of updated/deleted merge conflicts (#1040).

### 0.2.48

* gee pr_make: facilitate setting assignees for new PRs (#1024).
* gee: improve rebase --onto flow to reduce merge conflicts (#1022).
* gee commit: improve `--amend` behavior (#1021).
* gee: additional error checking for incorrect `gh repo set-default` configuration (#1017).

### 0.2.47

* gee lspr: add `--text` option (#1009).
* gee bisect: handle empty sets of commits (#1008).
* gee: handle closed PRs that are also marked as drafts (#1004).
* gee: add `GEE_ENABLE_PRESUBMIT_CANCEL` feature (#1001).
* gee: allow override of default tool paths (#991).

### 0.2.46

* gee bisect: a helpful utility for wrapping "git bisect" (#985).
* gee mkbr: Use reset when restoring a branch from origin instead of rebase (#983).
* gee: explicitly specify gcloud project when querying presubmit results (#981).
* gee bash_setup: disable window title control sequence if not running screen/tmux (#980).

### 0.2.45

* gee: add support for `GEE_DIR` and `GEE_REPO_DIR` environment variables (#974, #977)
* gee: handle case where "master" and "main" branches both exist (#948)

### 0.2.43

* gee setup: add support for BeyondCompare. (#945)
* gee migrate_default_branch: new command to ease migration from
  master to main branch names.  (#929)
* gee pr_submit: retry failed pull requests. (#870)
* gee checks: wait for slow-to-start tests. (#868)

### 0.2.42

* gee: make colors configurable.  Enhanced hidden "gee colortest" command.

### 0.2.41

* gee pr_submit: fix bug that was deleting the PR if `gh pr merge` command failed.
* gee: improve color scheme (#858)

### 0.2.40

* gee up: resolve conflicts when integrating commits from origin (#849)
* gee find: also traverses symlinks by default. (#847)

### 0.2.39

* gee find: new command, quickly find named files. (#839)
* gee vimdiff: new command, view local changes in a file. (#839)
* gee bazelgc: fix, correctly handle deletion of too many files. (#817)
* gee bazelgc: also prune old files from ~/.cache/bazel-disk-cache (#813)
* gee grep: new command, easily search a git branch using grep or ripgrep (#811)

### 0.2.38

Cosmetic improvements:

* Improve `pr_checks` output formatting (#800)
* Catch and handle error code from `gh pr list` (#794)
* Improved detection and reporting of failed subcommands (#792)

### 0.2.37

* Re-enable `--autostash` behavior when rebasing (#780)
* Improve error reporting (#778)
* `gee pr_checks`: report the buildbuddy URL on failure (#788)
* `gee recover_stashes`: helps users recover dropped stashes (#791)

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
