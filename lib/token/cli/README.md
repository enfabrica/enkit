# entoken utility

This utility allows to generate cryptographic token with the enkit libraries.

Build the command with: `bazelisk build :entoken`

Run the command manually, or with: `bazelisk run :entoken -- --help` to learn more.

To generate a symmetric key, and store it in a file, you can just:

    ./entoken symmetric generate -k file.key

To generate a pair of signing and verifying keys, you can use:

    ./entoken signing generate -s signing.key -f verifying.key

Of course, you can invoke the command directly with bazelisk, but remember to use absolute paths:

    bazelisk run :entoken -- signing generate -s /tmp/signing.key -f /tmp/verifying.key
