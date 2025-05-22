# github.com/golang/mock patches

`mock` is required by the buildbarn ecosystem, and that ecosystem applies
patches to `mock` upon import.

To avoid a WORKSPACE dependency cycle, these patches are copied here from
https://github.com/buildbarn/bb-remote-asset/tree/master/patches/com_github_golang_mock and https://github.com/buildbarn/bb-storage/tree/master/patches/com_github_golang_mock
