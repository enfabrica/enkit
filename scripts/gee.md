```
. __ _  ___  ___
 / _` |/ _ \/ _ \  git
| (_| |  __/  __/  enabled
 \__, |\___|\___|  enfabrication
 |___/
```

gee version: 0.2.29

gee is a wrapper around the "git" and "gh-cli" tools.  "gee" captures all
tribal knowledge about how to use git the right way (for us), implementing one
standard and well-supported workflow.  "gee" is also an instructional tool: by
showing each command as it executes, gee helps users learn git.

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

## Command Summary

| Command | Summary |
| ------- | ------- |
| <a href="#bash_setup">`bash_setup`</a> | Configure the bash environment for gee. |
| <a href="#cleanup">`cleanup`</a> | Automatically remove branches without local changes. |
| <a href="#codeowners">`codeowners`</a> | Provide detailed information about required approvals for this PR. |
| <a href="#commit">`commit`</a> | Commit all changes in this branch |
| <a href="#config">`config`</a> | Set various configuration options. |
| <a href="#create_ssh_key">`create_ssh_key`</a> | Create and enroll an ssh key. |
| <a href="#diff">`diff`</a> | Differences in this branch. |
| <a href="#fix">`fix`</a> | Run automatic code formatters over changed files only. |
| <a href="#gcd">`gcd`</a> | Change directory to another branch. |
| <a href="#get_parent">`get_parent`</a> | Which branch is this branch branched from? |
| <a href="#hello">`hello`</a> | Check connectivity to github. |
| <a href="#help">`help`</a> | Print more help about a command. |
| <a href="#init">`init`</a> | initialize a new git workspace |
| <a href="#log">`log`</a> | Log of commits since parent branch. |
| <a href="#lsbranches">`lsbranches`</a> | List information about each branch. |
| <a href="#make_branch">`make_branch`</a> | Create a new child branch based on the current branch. |
| <a href="#pack">`pack`</a> | Exports all unsubmitted changes in this branch as a pack file. |
| <a href="#pr_check">`pr_check`</a> | Checks whether presubmit tests for a PR. |
| <a href="#pr_checkout">`pr_checkout`</a> | Create a client containing someone's pull request. |
| <a href="#pr_edit">`pr_edit`</a> | Edit an existing pull request. |
| <a href="#pr_list">`pr_list`</a> | List outstanding PRs |
| <a href="#pr_make">`pr_make`</a> | Creates a pull request from this branch. |
| <a href="#pr_push">`pr_push`</a> | Push commits into another user's PR branch. |
| <a href="#pr_rollback">`pr_rollback`</a> | Create a rollback PR for a specified PR. |
| <a href="#pr_submit">`pr_submit`</a> | Merge the approved PR into the parent branch. |
| <a href="#pr_view">`pr_view`</a> | View an existing pull request. |
| <a href="#remove_branch">`remove_branch`</a> | Remove a branch. |
| <a href="#repair">`repair`</a> | Repair your gee workspace. |
| <a href="#restore_all_branches">`restore_all_branches`</a> | Check out all remote branches. |
| <a href="#revert">`revert`</a> | Revert specified files to match the parent branch. |
| <a href="#rupdate">`rupdate`</a> | Recursively integrate changes from parents into this branch. |
| <a href="#set_parent">`set_parent`</a> | Set another branch as parent of this branch. |
| <a href="#share">`share`</a> | Share your branch. |
| <a href="#unpack">`unpack`</a> | Patch the local branch from a pack file. |
| <a href="#update">`update`</a> | integrate changes from parent into this branch. |
| <a href="#update_all">`update_all`</a> | Update all branches. |
| <a href="#upgrade">`upgrade`</a> | Upgrade the gee tool. |
| <a href="#version">`version`</a> | Print tool version information. |
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

Usage: `gee log`

### diff

Usage: `gee diff [<files...>]`

Diffs the current branch against its parent branch.

If <files...> are omited, defaults to all files.

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

If a merge conflict occurs, "gee" drops the user into a sub-shell where the
user can either resolve the conflicts and continue, or abort the entire
operation.

### rupdate

Aliases: rup

Usage: `gee rupdate`

"gee rupdate" recursively rebases each branch onto it's parent.

If a merge conflict occurs, "gee" drops the user into a sub-shell where the
user can either resolve the conflicts and continue, or abort the entire
operation.  Note that the merge conflict can be in a parent of the current
branch.

### update_all

Aliases: up_all

Usage: `gee update_all`

"gee update_all" updates all local branches (in the correct order),
by rebasing child branches onto parent branches.

If a merge conflict occurs, "gee" drops the user into a sub-shell where the
user can either resolve the conflicts and continue, or abort the entire
operation.  Note that the merge conflict can be in a parent of the current
branch.

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

Usage: `gee pr_list [<user>]`

Lists information about PRs associated with the specified user (or yourself, if
no user is specified).

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

### pr_check

Aliases: pr_checks check_pr

Usage: `gee pr_check`

Checks presubmit tests.

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

Usage: `gee remove_branch <branch-name>`

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

Usage: `gcd [-b] <branch>[/<path>]`

The "gee gcd" command is not meant to be used directly, but is instead designed
to be called from the "gcd" bash function, which can be imported into your
environment with gee's "bash_setup" command, like this:

    eval "$(gee bash_setup)"

(This command should be added to your .bashrc file.)

Once imported, the "gcd" function can be used to change directory between
branches.

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

gcd also updates the following environment variables:

* BROOT always contains the path to the root directory of the current branch.
* BRBIN always contains the path to the bazel-bin directory beneath root.

### hello

Usage: `gee hello`

Verifies that the user can communicate with github using ssh.

For more information:
  https://docs.github.com/en/github/authenticating-to-github/connecting-to-github-with-ssh

### create_ssh_key

Usage: `gee create_ssh_key`

This command will attempt to re-enroll you for ssh access to github.

Normally, "gee init" will ensure that you have ssh access.  This command
is only available if something else has gone wrong requiring that keys
be updated.

### share

Usage: `gee share`

Displays URLs that you can paste into emails to share the contents of
your branch with other users (in advance of sending out a PR).

### repair

Usage: `gee repair <command>`

Gee tries to control some metadata and attempts to file away some of the
sharp edges from git.  Sometimes, bypassing gee to use git directly can
cause some of gee's metadata to become stale.  This command fixes up
any missing or incorrect metadata.

### restore_all_branches

Usage: `gee restore_all_branches`

Gee looks up all branches on the origin remote, and makes sure an equivalent
branch is checked out and updated locally.

Note that gee isn't able to restore parentage metadata in this way.  Be
sure to invoke `gee set_parent` in branches that benefit from this.

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

