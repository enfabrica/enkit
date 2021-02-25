# Enkit (engineering toolkit)

---
##Testing 

##### Setting up for tests 
> Non-bazel managed dependencies
1. google-cloud-sdk
    * install here https://cloud.google.com/sdk/docs/install
        PLEASE NOTE: do not install using snap/brew/apt-get etc
        emulators do not work
    * After following the instructions to install here
    * run the following command to get access to the emulators
        > gcloud components install beta 
    * Add the gcloud binary to the local binaries directory with the following symlink
        > ln -s $(which gcloud) /usr/local/bin
                    
2. Get a service account from <x, Y, Z person>
    * Put it in credentials/credentials.json     
                                                                                         >
##### Examples of Running Tests
* Running a specific go test target
> bazel test //astore/server/astore:go_default_test
* Running specific test of a test file 
> bazel test //astore/server/astore:go_default_test --test_filter=^TestServer$
* Running Everything 
> bazel test //...

##### Adding Tests
1. Create the test in * _test.go 
2. Run 
    > bazel run //:gazelle
* if your test needs server dependencies, such as astore or minio 
    1. Tests must be run as local = True 
    2. Test must also include the target "//credentials:credentials.json"

Clean Up / Dev Helpers  
Remove all emulator spawned processes
> ps aux | grep gcloud/emulators/datastore | awk '{print $2}' | xargs kill


##### Developing Tests


--- 
