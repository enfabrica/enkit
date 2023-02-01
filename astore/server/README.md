# How to test before deploying

TODO(ahungrynacho, ccontavalli): please tell me how to test this.

# How to deploy and use the astore server

1. Read the [README.md file in the credentials directory](credentials/),
   and follow the instructions there to the letter.

2. Run `bazelisk run :deploy`, and you should be set.
   If this returns errors the first time (likely), follow the instructions on
   the screen to fix them.

# Debugging

1. Visit the URL configured in the `credentials/site-url.flag` file. Does it work?

2. Can you see information in the logs, with `gcloud app logs tail -s default`?

3. Good luck.
