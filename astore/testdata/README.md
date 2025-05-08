# Initialization

1. **Fetch service account key** for testing:

   ```
   gcloud \
     --project=astore-284118 \
     secrets \
     versions \
     access \
     latest \
     --secret=astore_testing_service_account_key_json \
     > astore/testdata/credentials.json
   ```

1. **Run end-to-end tests**:

   ```
   bazel test //astore:astore_test
   ```
