# Credentials directory

There are 2 ways to provide credentials to the proxy server:

1) Provide the credentials at run time, with the correct flags.

2) Build the credentials into the binary before running it.
   Create a `.flag` file for each flag you want to compile into the binary at build time.

   For example, to build a default `credentials-file` into the binary, just store a `credentials-file.flag`
   in this directory, and build the binary again with `bazel` or `bazelisk`.
   The file has to be named after a flag name, with flag extension, and contains the raw value that the flag expects.

   Extra extensions are ignored, so if you prefer to call the file `signing-config.flag.json`, it is entirely
   up to you.

# Recommended files

* auth-url.flag - url to redirect the users to for web authentication.

* token-encrytion-key.flag - encryption key used by your authentication server. This is necessary so that the proxy can decode the token.

* token-verifying-key.flag - public key used to verify the signature of the token.

* sid-encryption-key.flag - key used to sign the session id for the user. If not supplied, a new key will be generated
  for each instance of the proxy. You must supply a key if you want to allow binary restarts to not cause all sessions from
  your users to be dropped, or if you want to run multiple proxies in the cloud or behind a load balancer.

  To generate a sid-encryption-key, you can use the tool under `lib/token/cli`, `entoken`. 
