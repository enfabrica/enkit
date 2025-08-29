DEFAULT_PATCHES = {
    "cloud.google.com/go/datastore": [
        "//internal/bzlmod/patches:com_google_cloud_go_datastore.patch",
    ],
    "github.com/buildbarn/bb-storage": [
        "//internal/bzlmod/patches:com_github_buildbarn_bb_storage.patch",
    ],
}