# Credentials directory

There are 2 ways to provide credentials to the astore server:

1) Provide the credentials at run time. Use command line flags like `--credentials-file` to
   specify the path of the file containing credentials.

2) Build the credentials into the binary before pushing it. Create a `.flag` file for each
   flag you want to compile into the binary at build time.
   For example, to build a default `credentials-file` into the binary, just store a `credentials-file.flag`
   in this directory, and build the binary again with `bazel` or `bazelisk`.
   The file has to be named after a flag name, with flag extension, and contains the raw value that
   the flag expects.

   For example, if the flag expects the path of a json file, and the flag is called `--signing-config`, you
   need to create a `signing-config.flag` in this directory, with the desired json file.

   Extra extensions are ignored, so if you prefer to call the file `signing-config.flag.json`, it is entirely
   up to you.

# Recommended files

Before trying to create the files, make sure you have created a project:

1. Go to the [google cloud console](http://console.cloud.google.com)
2. Use the pull down menu in the very top navigation bar to the left of the search bar to open a popup to select a project.
3. Click "NEW PROJECT".

To automatically deploy an astore server, the following files need to be provided:

* bucket.flag - name of the GCS bucket to use. This is where the artifacts will be stored on GCS.
  To create a bucket, go to: https://console.cloud.google.com/storage/create-bucket

* signing-config.flag.json - .json file containing the GCP key to use to generate signed URLs. 
  To create this file, you need to:

      1. Create a service account by clicking "CREATE SERVICE ACCOUNT" [here](https://console.cloud.google.com/iam-admin/serviceaccounts).
      2. Download the .json file with the key, and store it as signing-config.json.
      3. Authorize the service account to have access to your bucket. Go on the [storage browser](https://console.cloud.google.com/storage/browser),
         click on the newly created bucket, "PERMISSIONS" tab, "ADD" button, selecting the service account you create in point 1 as member
         (you may need to cut and paste the full name, service-account@....com), and then granting the "Storage Object Admin" privilege.

  Without a signing-config file, all uploads and downloads will fail. If no credentials-config file is specified,
  the server will fall back to use the credentials-file.

* secret-file.flag.json - .json file containing the secret identifying the application with the google oauth servers.
  To create this file:

      1. Go on http://developers.console.google.com, "Oauth Consent Screen", create an internal consent screen.
      2. On the same console, go on "Credentials", click on "+ Create Credentials" at the top, select "Oauth Client ID",
         pick "Web Application", configure the two URLs accordingly. For example, if you want to serve the authenticator
         as "auth.startup.com", you'll need to specify "https://auth.startup.com/" in Authorized JavaScript origins, and
         "https://auth.startup.com/e/" (note the appended "/e/", used internally by the server), as "Authorized redirect URIs".
      3. Download the generated keys.

* token-encryption-key.flag.bin - this is a file generated with the `entoken` utility. Run:

      bazelisk run //lib/token/cli:entoken -- symmetric generate -k /tmp/file.key
      cp -f /tmp/file.key ./astore/server/credentials/token-encryption-key.flag.bin

* token-signing-key.flag.bin, and token-verifying-key.flag.bin - also generated with the `entoken` utility. Run:

      bazelisk run //lib/token/cli:entoken -- signing generate -s /tmp/signing.key -f /tmp/verifying.key
      cp -f /tmp/signing.key ./astore/server/credentials/token-signing-key.flag.bin
      cp -f /tmp/verifying.key ./astore/server/credentials/token-verifying-key.flag.bin

* site-url.flag - this file contains the public URL users will use to reach the authentication server.
  Visit [appengine custom domain console](https://console.cloud.google.com/appengine/settings/domains) to
  configure your app to serve the desired domain.

  If you bought your domains via google, you can then create subdomains by using [the domain admin page](http://domains.google.com).
  Note that at time of writing (07/2020) there seems to be a bug in the console, by which subdomains like `auth.corp` cannot
  be typed. Cut & paste works.

* credentials-file.flag.json (optional) - .json file containing the GCP key to use to authenticate with
  GCS and datastore. This file is only necessary if you are running the server outside of google cloud.
  In facts, when running in Google App Engine or GCP, credentials are automatically provided in the environment.
  
  To generate this file, follow the same instructions to generate a signing-config file. It can be the same file,
  although it's a generally a good idea to keep them separate.

* project-id-file.flag.json (optional) - file containing the project-id to use when accessing the datastore API.
  This is only necessary if you are running the server outside of google cloud. You can see the project-id to
  use by looking at the first column of `gcloud projects list`.

* cookie-domain.flag (optional) - contains a domain name, which will be used to set the Domain option in
  authentication cookies, allowing those cookies to be shared within the domain. Due to how HTTP works,
  cookie-domain should be a parent domain or sub-domain of site-url, otherwise the browser will reject
  the option. As you authorize the authentication cookie to be shared within this domain, the auth server
  also assumes that it is ok to redirect the user back to one of those host names at the end of the
  authentication process.
