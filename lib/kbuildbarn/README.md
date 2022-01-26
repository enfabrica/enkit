This package is designed for integration between buildbarn and other first party applications written in enkit.

Buildbarn and other remote executors format their external tools around already defined .proto definitions. This library
is designed to be configurable based on external templates.

Examples:


    MyApplication --buildbarn-dsn=http://foo.internal
    Delivered: bytestream://127.0.0.1/blobs/curry/444
    ====== 
    hash, size, err := kbuildbarn.ByteStreamUrl("bytestream://127.0.0.1/blobs/curry/444")

    bbparam := kbuildbarn.NewBuildBarnParams("http://foo.internal", "foo.bar", hash, size)
    
    ======
    MyApplication.Upload(bbparams.FileUrl())

