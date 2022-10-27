# Enkit (engineering toolkit)

## Importing into a downstream Bazel workspace

When using `enkit` in a downstream workspace, there are two options for
loading Go dependencies:

- Call only `//bazel:go_repositories%go_repositories` from this repo, and use
  only those dependencies in the downstream repo. This works if Golang
  binaries only need to be built from this repo and not the downstream repo.

- Call `//bazel:go_repositories%go_repositories` **after** loading Golang
  dependencies in the downstream repo. This would ensure that:

  1. Go dependencies in the downstream repo obey that repo's go.mod version
     selection, rather than enkit's, for minimal surprises.

  1. enkit dependencies are loaded, for processes that require a complete
     dependency graph

  Note that building binaries from enkit loaded in a downstream repo in this
  manner will not necessarily match those built from this repo directly, as
  the downstream repo may be loading different versions of enkit's
  dependencies. This may cause build divergence and/or failures.

## Testing

### Setting up for tests

#### Install non-bazel managed dependencies

1. `google-cloud-sdk`

   - Install here https://cloud.google.com/sdk/docs/install

     PLEASE NOTE: do not install using snap/brew/apt-get etc., as emulators do
     not work.

   - Run the following command to get access to the emulators:

     ```
     gcloud components install beta
     ```

   - Add the gcloud binary to the local binaries directory with the following
     symlink:

     ```
     ln -s $(which gcloud) /usr/local/bin
     ```

1. Get a service account from \<x, Y, Z person>

   - Put it in `//astore/testdata/credentials.json`

### Examples of Running Tests

- Running a specific go test target

  ```
  bazel test //astore:go_default_test
  ```

- Running specific test of a test file

  ```
  bazel test //astore:go_default_test --test_filter=^TestServer$
  ```

- Running Everything

  ```
  bazel test //...
  ```

- Remove all emulator spawned processes

  Sometimes emulator processes can be left behind after a test run. These can
  be cleaned up with:

  ```
  ps aux | grep gcloud/emulators/datastore | awk '{print $2}' | xargs kill
  ```

### Adding Tests

1. Create the test in `* \_test.go`

1. Run Gazelle:

   ```
   bazel run //:gazelle
   ```

1. If your test needs server dependencies, such as astore or minio, add the
   attribute `local = True` to the test rule.
