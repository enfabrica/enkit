# golang.org/x/oauth2 patches

`golang.org/x/oauth2` is required by the buildbarn ecosystem, and that ecosystem applies
patches to `golang.org/x/oauth2` upon import.

To avoid a WORKSPACE dependency cycle, these patches are copied here from
https://github.com/buildbarn/bb-storage/tree/master/patches/org_golang_x_oauth2.
