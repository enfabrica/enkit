# rules_antlr patches

`rules_antlr` is required by the buildbarn ecosystem, and that ecosystem applies
patches to `rules_antlr` upon import.

To avoid a WORKSPACE dependency cycle, these patches are copied here from
https://github.com/buildbarn/go-xdr/tree/master/patches/rules_antlr.
