```
. __ _  ___  ___
 / _` |/ _ \/ _ \  git
| (_| |  __/  __/  enabled
 \__, |\___|\___|  enfabrication
 |___/
```

gee version: 0.2.50

gee is a user-friendly wrapper (aka "porcelain") around the "git" and "gh-cli"
tools  gee is an opinionated tool that implements a specific, simple, powerful
workflow.

"gee" is also an instructional tool: by showing each command as it executes,
gee helps users learn git.

## Features:

Uses the "worktree" feature so that:

* every branch is always visible in its own directory.
* switching branches is accomplished by changing directory.
* it's harder to accidentally save changes to the wrong branch.
* users can have uncommitted changes pending in more than one branch.

All branch directories are named ~/gee/<REPO>/<BRANCH>.

All local commits are automatically backed up to github.

Tracks "parentage" (which branch is derived from which).

Sets up and enforces use of ssh for all interactions with github.

Supports multi-homed development (user can do work on various hosts without
NFS-mounted home directories).

## An example of simple use:

1. Run "gee init" to clone and check out the enfabrica/internal repo.  This
   only needs to be done once per home directory.

2. "cd ~/gee/internal/main" to start in the main branch.

3. Make a feature branch: "gee make_branch my_feature"
   Then: "cd $(gee gcd my_feature)"

4. Make some changes, and call "gee commit" whenever needed to checkpoint
   your work.

5. Call "gee update" to pull new changes from upstream.

6. When ready to send your change out for review:

```shell
    gee fix  # runs all automatic code formatters
    gee commit -a -m "ran gee fix"
    gee make_pr  # creates a pull request.
```

7. You can continue to make updates to your branch, and update your
   PR by running "gee commit".

8. When approved, run "gee submit_pr" to merge your change.

## An example of more complex use:

You can continue to develop a second feature while the first feature is out for
review.

1. Make a branch of a branch:

```shell
     cd $(gee gcd my_feature)
     gee mkbr my_feature2
```

2. Do work in the child branch:

```shell
     cd $(gee gcd my_feature2)
```

3. Recursively update a chain of branches:

```shell
     gee rupdate
```

## Environment variables

`gee`'s behavior can be moderated by setting the following variables in your shell environment:

* `GHUSER`: Your github username.

* `GEE_DIR`: By default, gee will place all work directories in `~/gee`.  This variable can
  be set to a different path to override this behavior.  `gee` will still create a repository
  directory beneath `$GEE_DIR` (ie, `~/gee/internal`), so if you want to change the repository
  directory as well, use the GEE_REPO and GEE_REPO_DIR environment variables.

* `GEE_REPO_DIR`: By default, gee will place a repository's workdirs in a directory specified
  by `${GEE_DIR}/${GEE_REPO}` where `GEE_DIR` defaults to `~/gee` and `GEE_REPO` defaults to `internal`.
  The `GEE_REPO_DIR` can be set to specify a different directory.  However, if `GEE_REPO_DIR` is set,
  you must also set the `GEE_REPO` environment variable.

* `GEE_REPO`: By default, gee will attempt to guess which repository to use based on the current
  working directory (ie. the `enkit` repo is inferred from `~/gee/enkit/master`).  To override this
  behavior (or, if you are using `GEE_REPO_DIR` to use a non-standard directory structure), set
  the `GEE_REPO` variable to the repository name (ie. `internal`, `enkit`, etc.).

* `GEE_BUILDLOGS_PROJECT`: If you are using gcloud to store your presubmit testing logs, you can
  override gee's default gcloud project by setting this environment variable.

* `UPSTREAM`: The name of the github user hosting the specified repository.  By default, `enfabrica`.

* `YESYESYES`: If set to a non-zero integer, will cause all yes/no prompts within `gee` to automatically
  select "yes".

* `gee` looks in a few places to find the tools it needs, but if gee has a hard time finding the
  right version of a tool, you can force `gee` to use a specific path by setting any or all
  of the variables `GIT`, `JQ`, `GH`, and `ENKIT`.

* The following environment variables can be set to a curses color value to override gee's default
  color scheme.  (The `gee colortest` command can be used to dump a color table and examples of the
  current color scheme.)

  * `GEE_COLOR_CMD_FG`
  * `GEE_COLOR_CMD_BG`
  * `GEE_COLOR_BANNER_FG`
  * `GEE_COLOR_BANNER_BG`
  * `GEE_COLOR_DBG_FG`
  * `GEE_COLOR_DBG_BG`
  * `GEE_COLOR_DIE_FG`
  * `GEE_COLOR_DIE_BG`
  * `GEE_COLOR_WARN_FG`
  * `GEE_COLOR_WARN_BG`
  * `GEE_COLOR_INFO_FG`
  * `GEE_COLOR_INFO_BG`

* `VERBOSE`: If set to a non-zero integer, will cause additional debug information to be logged.
  For developer use only.

* `GEE_ENABLE_PRESUBMIT_CANCEL`: If set to any value other than 0, will cause
  gee to cancel any running presubmit job when pushing a change to remote
  branch with an open PR.  Setting to 0 will disable this functionality.  This
  feature is no longer experimental and now defaults to enabled.

See also: `gee help bash_setup` for more environment variables to help you customize the git-aware
prompt that `gee bash_setup` makes available.

## Command Summary

| Command | Summary |
| ------- | ------- |
| <a href="#bash_setup">`bash_setup`</a> | Configure the bash environment for gee. |
| <a href="#bazelgc">`bazelgc`</a> | Garbage collect your bazel cache. |
| <a href="#bisect">`bisect`</a> | Find a commit that caused a command to fail. |
| <a href="#cleanup">`cleanup`</a> | Automatically remove branches without local changes. |
| <a href="#codeowners">`codeowners`</a> | Provide detailed information about required approvals for this PR. |
| <a href="#commit">`commit`</a> | Commit all changes in this branch |
| <a href="#config">`config`</a> | Set various configuration options. |
| <a href="#copy">`copy`</a> | Copy files (preserving history). |
| <a href="#create_ssh_key">`create_ssh_key`</a> | Create and enroll an ssh key. |
| <a href="#diagnose">`diagnose`</a> | Capture diagnostics about your repository. |
| <a href="#diff">`diff`</a> | Differences in this branch. |
| <a href="#find">`find`</a> | Finds a file by name in the current branch. |
| <a href="#fix">`fix`</a> | Run automatic code formatters over changed files only. |
| <a href="#gcd">`gcd`</a> | Change directory to another branch. |
| <a href="#get_parent">`get_parent`</a> | Which branch is this branch branched from? |
| <a href="#grep">`grep`</a> | Greps the current branch. |
| <a href="#hello">`hello`</a> | Check connectivity to github. |
| <a href="#help">`help`</a> | Print more help about a command. |
| <a href="#init">`init`</a> | initialize a new git workspace |
| <a href="#log">`log`</a> | Log of commits since parent branch. |
| <a href="#lsbranches">`lsbranches`</a> | List information about each branch. |
| <a href="#make_branch">`make_branch`</a> | Create a new child branch based on the current branch. |
| <a href="#migrate_default_branch">`migrate_default_branch`</a> | Migrate to a new default branch. |
| <a href="#pack">`pack`</a> | Exports all unsubmitted changes in this branch as a pack file. |
| <a href="#pr_cancel">`pr_cancel`</a> | Cancels any running gcloud builds associated with this branch. |
| <a href="#pr_checkout">`pr_checkout`</a> | Create a client containing someone's pull request. |
| <a href="#pr_check">`pr_check`</a> | Checks the status of presubmit tests for a PR. |
| <a href="#pr_edit">`pr_edit`</a> | Edit an existing pull request. |
| <a href="#pr_list">`pr_list`</a> | List outstanding PRs |
| <a href="#pr_make">`pr_make`</a> | Creates a pull request from this branch. |
| <a href="#pr_push">`pr_push`</a> | Push commits into another user's PR branch. |
| <a href="#pr_rerun">`pr_rerun`</a> | Rerun presubmit checks. |
| <a href="#pr_rollback">`pr_rollback`</a> | Create a rollback PR for a specified PR. |
| <a href="#pr_submit">`pr_submit`</a> | Merge the approved PR into the parent branch. |
| <a href="#pr_view">`pr_view`</a> | View an existing pull request. |
| <a href="#recover_stashes">`recover_stashes`</a> | Recover identifiers for old, dropped stashes. |
| <a href="#remove_branch">`remove_branch`</a> | Remove a branch. |
| <a href="#repair">`repair`</a> | Repair your gee workspace. |
| <a href="#restore_all_branches">`restore_all_branches`</a> | Check out all remote branches. |
| <a href="#revert">`revert`</a> | Revert specified files to match the parent branch. |
| <a href="#rupdate">`rupdate`</a> | Recursively integrate changes from parents into this branch. |
| <a href="#set_parent">`set_parent`</a> | Set another branch as parent of this branch. |
| <a href="#share">`share`</a> | Share your branch. |
| <a href="#unpack">`unpack`</a> | Patch the local branch from a pack file. |
| <a href="#update_all">`update_all`</a> | Update all branches. |
| <a href="#update">`update`</a> | integrate changes from parent into this branch. |
| <a href="#upgrade">`upgrade`</a> | Upgrade the gee tool. |
| <a href="#version">`version`</a> | Print tool version information. |
| <a href="#vimdiff">`vimdiff`</a> | Runs vimdiff to compare changes in a file. |
| <a href="#whatsout">`whatsout`</a> | List locally changed files in this branch. |

## Commands

### init

Usage: `gee init [<repo>]`

Arguments:

   repo: Specifies which enfabrica repository to check out.
         If repo is not specified, `internal` is used by default.

`gee init` creates a new gee-controlled workspace in the user's home directory.
The directory `~/gee/<repo>/main` will be created and populated, and all
other branches will be checked out into `~/gee/<repo>/<branch>`.

### config

Usage: `gee config <option>`

Valid configuration options are:

* "default": Reset to default settings.
* "enable_vim": Set "vimdiff" as your merge tool.
* "enable_emacs": Set "emacs" as your merge tool.
* "enable_vscode": Set "vscode" as your GUI merge tool.
* "enable_meld": Set "meld" as your GUI merge tool.
* "enable_bcompare": Set "BeyondCompare" as your GUI merge tool.

### make_branch

Aliases: mkbr

Usage: `gee make_branch <branch-name> [<commit-ish>]`
Aliases: mkbr

Create a new branch based on the current branch.  The new branch will be located in the
directory:
  ~/gee/<repo>/<branch-name>

If <commit-ish> is provided, sets the HEAD of the newly created branch to that
revision.

### log

Usage: `gee log [<args...>]`

Invokes `git logp` with the supplied arguments.

If the supplied arguments do not contain a commit range, then gee will show the
log messages for commits between the parent branch and the the current branch.

For example:

    gee log                # show all commits since HEAD of parent branch.

    gee log ./scripts/gee  # show commits since parent for a single file.

    gee log master...mybr  # show all commits in a specific range

### diff

Usage: `gee diff [<files...>]`

Shows all local changes this since branch diverged from its parent branch.

If <files...> are omited, shows changes to all files.

### find

Usage: `gee find [options] <expression>`

Searches the current branch for a file whose name matches the specified
expression.  Will initially search for the file without traversing symlinks.
If it fails to find any files, it will search again in the bazel-bin
subdirectory (and follow symlinks) to see if the file is generated by a bazel
rule.

Roughly equivalent to running:

    find -L "$(git rev-parse --show-toplevel)" -name .git -prune -or \
         -name "${expression}" -print

Example of use:

    gee find WORKSPACE

### grep

Usage: `gee grep [options] <expression>`

Searches the current branch for the specified expression.

If the ripgrep tool is installed, then this command invokes:

    rg "$@" "${BRANCH_ROOT_DIR}"

If ripgrep is not installed, this command falls back on the grep utility,
invoked to be similar in operation to ripgrep, like this:

    grep -r --exclude-dir=.git "$@" "${BRANCH_ROOT_DIR}"

If the `gee bash_setup` environment is loaded, `grg` is an alias for this
command.

Example of use:

    grg -l fdst

### vimdiff

Usage: `gee vimdiff <filename>`

Invokes vimdiff to show and edit the changes to a specific file in the current
branch, versus the version in the parent branch.  This can be useful to clean
up local changes, especially after resolving merge conflicts.

If installed, neovim will be used.  Otherwise, gee will fallback to vim.

When working in a branch created with `pr_checkout`, the parent branch isn't
a local worktree, and so vimdiff will produce an error and fail.

Example of use:

    gee vimdiff BUILD.bazel

### pack

Usage: `gee pack [-c] [-o <file.pack>]`

Creates a pack file containing all unsubmitted changes in this branch.

Flags:
  -o  Specifies a file to write to, instead of stdout.

### unpack

Usage: `gee unpack <file.pack>`

"unpack" attempts to patch the current branch from a pack file.

### update

Aliases: up

Usage: `gee update`

"gee update" attempts to rebase this branch atop its parent branch.

gee implements an interactive flow to resolve merge conflicts.  At each
conflict, you will be given the option of choosing:

* (M) for "mergetool", to invoke a mergetool (by default, vimdiff) to resolve
  the merge.
* (G) for "gui", to invoke a GUI mergetool (by default, meld).
* (O) for "old", to discard your changes and preserve the upstream change.
* (N) for "new", to discard the upstram change and use your change instead.
* (A) for "abort", to abort the rebase operation and return to your initial
      state.

gee always creates a `<branch-name>.REBASE_BACKUP` tag before updating your
branch.  If something went wrong with the merge and want to get back to where
you started, you can always run:

    git reset --hard your_branch.REBASE_BACKUP

to undo the last rebase operation.

### rupdate

Aliases: rup

Usage: `gee rupdate`

"gee rupdate" recursively rebases each branch onto it's parent.

gee implements an interactive flow to resolve merge conflicts.  At each
conflict, you will be given the option of choosing:

* (M) for "mergetool", to invoke a mergetool (by default, vimdiff) to resolve
  the merge.
* (G) for "gui", to invoke a GUI mergetool (by default, meld).
* (O) for "old", to discard your changes and preserve the upstream change.
* (N) for "new", to discard the upstram change and use your change instead.
* (A) for "abort", to abort the rebase operation and return to your initial
      state.

gee always creates a `<branch-name>.REBASE_BACKUP` tag before updating your
branch.  If something went wrong with the merge and want to get back to where
you started, you can always run:

    git reset --hard your_branch.REBASE_BACKUP

to undo the last rebase operation.

### update_all

Aliases: up_all

Usage: `gee update_all`

"gee update_all" updates all local branches (in the correct order),
by rebasing child branches onto parent branches.

gee implements an interactive flow to resolve merge conflicts.  At each
conflict, you will be given the option of choosing:

* (M) for "mergetool", to invoke a mergetool (by default, vimdiff) to resolve
  the merge.
* (G) for "gui", to invoke a GUI mergetool (by default, meld).
* (O) for "old", to discard your changes and preserve the upstream change.
* (N) for "new", to discard the upstram change and use your change instead.
* (A) for "abort", to abort the rebase operation and return to your initial
      state.

gee always creates a `<branch-name>.REBASE_BACKUP` tag before updating your
branch.  If something went wrong with the merge and want to get back to where
you started, you can always run:

    git reset --hard your_branch.REBASE_BACKUP

to undo the last rebase operation.

### whatsout

Usage: `gee whatsout`

Reports which files in this branch differ from parent branch.

### codeowners

Aliases: owners reviewers

Usage: `gee codeowners [--comment]`

Gee examines the set of modified files in this branch, and compares it against
the rules in the CODEOWNERS file.  Gee then presents the user with detailed
information about which approvals are necessary to submit this PR:

*  Each line contains a list of users who are code owners of at least
   some part of the PR.

*  A minimum of one user from each line must provide approval in order for the
   PR to be merged.

If `--comment` option is specified, the codeowners information is appended to the
current PR as a new comment.

### lsbranches

Aliases: lsb lsbr

Usage: `gee lsbranches`

List information about all branches.

NOTE: the output of this command is likely to change in the near future.

### cleanup

Usage: `gee cleanup`

Automatically removes branches without local changes.

### get_parent

Usage: `gee get_parent`

### set_parent

Usage: `gee set_parent <parent-branch>`

Gee keeps track of which branch each branch is branched from.  You can
change the parent of the current branch with this command, but be sure
to do a "gee update" operation afterwards.

### commit

Aliases: push c

Usage: `gee commit [<git commit options>]`

Commits all outstanding changes to your local branch, and then immediately
pushes your commits to `origin` (your private, remote github repository).

"commit" can be used to checkpoint and back up work in progress.

Note that if you are working in a PR-associated branch created with `gee
pr_checkout`, your commits will be pushed to your `origin` remote, and the
remote PR branch.  To contribute your changes back to another user's PR branch,
use the `gee pr_push` command.

Unless GEE_ENABLE_PRESUBMIT_CANCEL feature is disabled, gee
will check to see if pushing the current commit will invalidate a presubmit job
in the `pending` state.  If this is the case, gee will kill the previous
presubmit before pushing the changes and thus kicking off the new presubmit.

Example:

    gee commit -m "Added \"gee commit\" command."

See also:

* pr_push

### revert

Usage: `gee revert <files...>`

Reverts changes to the specified files, so that they become identical to the
version in the parent branch.  If the file doesn't exist in the parent
branch, it is deleted from the current branch.

Example:

    gee revert foobar.txt

### pr_checkout

Usage: `gee pr_checkout <PR>`

Creates a new branch containing the specified pull request.

Note that the new will be configured so that `gee update` will update that
branch by integrating changes from the original pull request.  However,
`gee commit` will still only push commits to your own local and `origin`
repositories.  If you want to push commits back into the original PR,
use the `pr_push` command.

See also:

* commit
* pr_push

### pr_push

Usage: `gee pr_push`

`gee pr_push` must be executed from within a branch created by `gee pr_checkout`.

`gee commit` will create a local commit, and push that commit to `origin`, the
remote repository owned by you.  `gee pr_push` can then be used to also push
your commits into another user's remote pull request branch.

`gee pr_push` will refuse to proceed unless all changes from the remote pull
request branch are already integrated in your local branch, so you might need
to `gee update` before `gee pr_push`.

After pushing your changes into another user's PR branch, be sure to directly
notify that user, so they know to pull your changes into their local branch.
Otherwise, the other user might accidentally lose your commits entirely if they
force-push.  Remote users can integrate your changes using the `gee update`
command, or `git rebase --autostash origin/<branch>` if they aren't a gee user.

See also:

* commit
* pr_checkout

### pr_list

Aliases: lspr list_pr prls

Usage: `gee pr_list [--text] [<user>]`

Lists information about PRs associated with the specified user (or yourself, if
no user is specified).

The `--text` option provides an alternative formatting for a list of open PRs, more
suitable for pasting into an email.

Example:

    $ gee lspr jonathan-enf
    PRs associated with this branch:
    OPEN 1181 codegen tool

    Open PRs authored by jonathan-enf:
    #1205   REVIEW_REQUIRED Fix libsystemc build file error.
    #1181   REVIEW_REQUIRED codegen tool
    #1158   REVIEW_REQUIRED Added @gmp//:libgmpxx
    #1148   REVIEW_REQUIRED Added gee to enkit.config.yaml.
    #1136   REVIEW_REQUIRED Unified PtrQueue and Queue implementations.
    #1130   REVIEW_REQUIRED Owners of /poc/{sim,models}
    #1059   REVIEW_REQUIRED CSV file helper library

    PRs pending their review:
    #1200  taoliu0  2021-08-12T15:26:03Z  Added an example integrating SC

An example of using the "--text" option:

    $ /home/jonathan/gee/enkit/gee_lspr_format/scripts/gee lspr --text
    * #29644: Lorem ipsum dolor sit amet
      2023-12-30 APPROVED Checks passed.
      https://github.com/enfabrica/internal/pull/29644

    * #29641: consectetur adipiscing elit
      2023-12-30 REVIEW_REQUIRED DRAFT Checks passed.
      https://github.com/enfabrica/internal/pull/29641

    * #29640: sed do eiusmod tempor incididunt
      2023-12-30 APPROVED Checks passed.
      https://github.com/enfabrica/internal/pull/29640

    * #29625: ut labore et dolor magna aliqua
      2023-12-29 REVIEW_REQUIRED Checks passed.
      https://github.com/enfabrica/internal/pull/29625

### pr_edit

Aliases: edpr pred edit_pr

Usage: `gee edit_pr <args>`

Edit an outstanding pull request.

All arguments are passed to "gh pr edit".

### pr_view

Aliases: view_pr

Usage: `gee pr_view`

View an outstanding pull request.

### pr_make

Aliases: mail send pr_create create_pr make_pr mkpr prmk

Usage: `gee make_pr <gh-options>`

Creates a new pull request from this branch.  The user will be asked to
edit a PR description file before the PR is created.

If you have any second thoughts during this process: Adding the token "DRAFT"
to your PR description will cause the PR to be marked as a draft.  Adding the
token "ABORT" will cause gee to abort the creation of your PR.

Uses the same options as "gh pr create".

### pr_cancel

Aliases: cancel cancel_pr

Usage: `gee pr_cancel`

Cancels any pending (status = QUEUED or WORKING) gcloud builds jobs
associated with this branch.  This command can be used to cancel
a set of presubmits that were triggered by a change to this branch.

### pr_check

Aliases: pr_checks check_pr check checks

Usage: `gee pr_check [--wait]`

Returns the state of presubmit checks.  If the --wait option is provided,
this command will continue to report check status until all pending
checks have completed.

### pr_rerun

Aliases: pr_run rerun gcbrun

Usage: `gee pr_rerun`

Forces any presubmit checks associated with this PR to try again.

This normally occurs whenever you push a new commit to your PR.  Sometimes,
however, a presubmit will fail for reasons unrelated to your PR (for example, a
transient infrastructure failure).  This command allows you to re-run the set
of presubmit checks without changing your PR.

### pr_submit

Aliases: merge submit_pr

Usage: `gee submit_pr`

Merges an approved pull request.

### pr_rollback

Usage: `gee pr_rollback <PR>`

Creates a branch named `rollback_<PR>`, attempts to revert the commit
associated with that PR, and if successful, creates a new PR that rolls
back the specified PR.

Example:

    gee pr_rollback 1234

### remove_branch

Aliases: rmbr

Usage: `gee remove_branch <branch-names...>`

Removes a branch and it's associated directory.

### fix

Usage: `gee fix [<files>]`

Looks for a "fix_format.sh" script in the root directory of the current branch,
and runs it.  This script runs a set of language formatting tools over either:

  - the files specified on the command line, or
  - if no files are specified, all of the locally changed files in this
    branch.

Note: "gee fix" (which fixes code in a branch) is different from "gee repair"
(which checks the gee directory for errors and repairs them).

Note: "fix_format.sh" used to be integrated into gee, but has been separated
out as formatting rules are highly project specific.

### gcd

Usage: `gcd [-b] [-m] <branch>[/<path>]`

Print the path to an equivalent directory in another worktree (branch).
This command is meant to be invoked from the "gcd" bash function, which
invokes this command and then chdir's into that directory.

The "gcd" bash function can be imported into your shell with gee's "bash_setup"
command, like this:

    eval "$(gee bash_setup)"

(This command should be added to your .bashrc file.)

Options:

* "-b" causes gee to create a new branch if the specified branch doesn't
  exist.  The new branch is a child of the current branch.
* "-m" causes gee to create a new branch if the specified branch doesn't
  exist.  The new branch is a child of the master (or main) branch.

If only "<branch>" is specified, "gcd" will change directory to the same
relative directory in another branch.  If "<branch>/<path>" is specified,
"gcd" will change directory to the specified path beneath the specified
branch.

For example:

    cd ~/gee/enkit/branch1/foo/bar
    # now in ~/gee/enkit/branch1/foo/bar
    gcd branch2
    # now in ~/gee/enkit/branch2/foo/bar
    gcd branch3/foo
    # now in ~/gee/enkit/branch3/foo
    gcd -b branch4
    # now in branch4/foo, a child branch of branch3.
    gcd -m new_feature
    # now in new_feature/foo, a child branch of master.

The "gcd" function also updates the following environment variables:

* BROOT always contains the path to the root directory of the current branch.
* BRBIN always contains the path to the bazel-bin directory beneath root.

### hello

Usage: `gee hello`

Verifies that the user can communicate with github using ssh and the
github cli interface.

For more information:
  https://docs.github.com/en/github/authenticating-to-github/connecting-to-github-with-ssh

### create_ssh_key

Usage: `gee create_ssh_key`

This command will attempt to re-enroll you for ssh access to github.

Normally, "gee init" will ensure that you have ssh access.  This command
is only available if something else has gone wrong requiring that keys
be updated.

### copy

Aliases: cp

Usage: `gee copy <files...> <destination>`

Creates a copy of one or more files, preserving git history.  (Just
copying a file using cp will cause git to assume a new file, and
not preserve history.)

This operation will create a new commit containing the copy operation.
All files being copied should be "clean" (committed, not staged) for
this to work correctly.

Destination may either be a filename or a directory.  When copying
more than one file, destination must be a directory.

Examples:

    gee copy foo.txt bar.txt
    mkdir bardir
    gee copy foo.txt bar.txt bardir/

### share

Usage: `gee share`

Displays URLs that you can paste into emails to share the contents of
your branch with other users (in advance of sending out a PR).

### bazelgc

Usage: `gee bazelgc`

Identifies a set of bazel cache directories that are no longer associated with
any worktree (branch) that gee knows about, and offers to delete them.

### bisect

Usage: `gee bisect [--good <commit-ish>] command...`

This command wraps the `git bisect` command, and attempts to discover
a commit that causes the provided command to transition from a success
to a failure.  During this process, gee will create a special branch
named `bisect_<branchname>` to perform the bisect operation in.

If "--good" is specified, gee will using the specified commit as the starting
point for bisect (the first good commit, where the command is expected to
succeed).  If "--good" is not used, gee will attempt to find a previous last
good commit by testing a day, a week, a month, 3 months, and finally six months
into the past.  Once a past good commit is found, the `git bisect` command will
be used to identify the commit that caused a transition from a pass to a fail.

gee assumes that the provided command will fail at the head revision.
If this is not the case, the behavior of this command is undefined.

### diagnose

Usage: `gee diagnose`

This command produces a `~/gee.diagnostics.txt` file that might
be useful to share with the tool maintainers if something has gone
wrong with your gee repository.

### migrate_default_branch

Usage: `gee migrate_default_branch`

This command performs necessary local fix-ups after an upstream
branch migrates their default branch from an old name (ie, "master")
to a new name (ie, "main").

### repair

Usage: `gee repair <command>`

Gee tries to control some metadata and attempts to file away some of the
sharp edges from git.  Sometimes, bypassing gee to use git directly can
cause some of gee's metadata to become stale.  This command fixes up
any missing or incorrect metadata.

Additionally, "gee repair" automatically invokes "gee diagnose," which
produces a diagnostics file in `~/gee.diagnostics.txt`.

### restore_all_branches

Usage: `gee restore_all_branches`

Gee looks up all branches on the origin remote, and makes sure an equivalent
branch is checked out and updated locally.

If you have the `dialog` utility installed, gee will let you interactively
select which branches you want to restore.

Alternately, if the user supplies a list of branches on the command line (for
instance, "origin/test1 origin/test2"), restore_all_branches will only attempt
to restore those branches.

Note that gee isn't able to restore parentage metadata in this way.  Be
sure to invoke `gee set_parent` in branches that benefit from this.

### recover_stashes

Usage: `recover_stashes <"optional string to search for">`

The `recover_stashes` command is useful if you accidentally dropped some
stashed changes that you wanted to hold onto.  Even dropped stashes
can be recovered if you know the right hash for the stash, and this
command helps you discover them.

By default, `recover_stashes` will print the hash ids for all stashes
in your repo, sorted by author-date.  Any additional options are used
to filter the commits (ie. by filename or file contents).

Results are reported as a list of commits and their author-dates.

For example:

    $ gee recover_stashes enkit
    Created temporary FSCKFILE=/tmp/stashes-jonathan.0JTAXI
    Searching your repo for discarded stash commits...
    Checking object directories: 100% (256/256), done.
    Checking objects: 100% (8266/8266), done.
    Cached search results to FSCKFILE=/tmp/stashes-jonathan.0JTAXI
    Found 58 commits.
    Filtering candidate commits...
    Found 4 matches.
      914c9f7f2a43e34c8b1f8ac26f478d4a3ae66c46 Sat Sep 24 12:56:57 2022 -0700
      7b81aec0ed7806963e79bd2b236523aba57405a7 Tue Oct 4 07:33:03 2022 +0000
      b957986e8044e8b3097591bae25dfee04e567469 Tue Oct 4 09:32:05 2022 -0700
      184e791a27fe0f2dd3a7c87a7a9ea4c384d498e7 Thu Oct 6 06:01:44 2022 +0000
    Remove /tmp/stashes-jonathan.0JTAXI if you no longer need it.

### bash_setup

Usage: `eval "$(~/bin/gee bash_setup)"`

The "bash_setup" command emits a set of bash aliases and functions that
streamline the use of gee.  The following functions are exported:

  "gee": invokes "gee $@"
  "gcd": rapidly change between gee branch directories.

Additionally, the following functions can be used to customize your
command prompt with useful information about your git work tree:
  "gee_prompt_git_info": prints git-related information suitable for
    integrating into your own prompt.
  "gee_prompt_print": Prints a string suitable for using as a prompt.
  "gee_prompt_set_ps1": Sets PS1 to the output of gee_prompt_print.

This custom git-aware prompt will keep you apprised of which git branch you are
in, and will also tell you important information about the status of your
branch (whether or not you are in the middle of a merge or rebase operation,
whether there are uncommitted changes, and more).

The easiest way to make use of the git-aware prompt is to modify your .bashrc
file to set PROMPT_COMMAND to "gee_prompt_set_ps1", as shown below:

    export PROMPT_COMMAND="gee_prompt_set_ps1"

This prompt can be customized by setting the following environment variables:

*  GEE_PROMPT: The PS1-style prompt string to put at the end of every prompt.
    Default:  `$' [\\!] \\w\033[0K\033[0m\n$ '`
   More info: https://www.man7.org/linux/man-pages/man1/bash.1.html#PROMPTING
*  GEE_PROMPT_BG_COLOR: The ANSI color to use as the background (default: 5).
*  GEE_PROMPT_FG1_COLOR: The foreground color for git-related info (default: 9).
*  GEE_PROMPT_FG2_COLOR: The foreground color for GEE_PROMPT (default: 3).

Easter egg: Use the "gee_prompt_test_colors" command to view a test pattern
of the basic 4-bit ANSI color set.

If you need further customization, you are encouraged to write your own
version of gee_prompt_set_ps1.

Also sets GEE_BINARY to point to this copy of gee.

### upgrade

Usage: `gee upgrade [--check]`

### version

Usage: `gee version`

### help

Usage: `gee help [<command>|usage|commands|markdown]`

The "usage" option produces gee's manual.

The "commands" option shows a summary of all available commands.

The "markdown" option produce's gee's manual, in markdown format.

