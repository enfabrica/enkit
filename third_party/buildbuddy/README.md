# BuildBuddy Protos

This directory contains a subset of code from
https://github.com/buildbuddy-io/buildbuddy.

Imports are managed by Copybara; any changes should be made not to the code
files directly, but to the Copybara workflow `copy.bara.sky` in this directory,
and then the workflow used to produce a Github PR with the desired changes.

Copybara controls:

* The ref of source fetched
* The files fetched from the source, and the files modified in the destination
* Any code transformations (moves, regex replacements, etc.)

## Importing via Copybara

1. Install Copybara - see https://github.com/google/copybara for instructions.

1. Run Copybara, providing a new branch name to use for the PR:

   ```
   copybara third_party/bazel/copy.bara.sky --github-destination-pr-branch your_branch_name
   ```