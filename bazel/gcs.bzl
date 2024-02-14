def gcs(name, srcs, gcs_bucket="no_bucket", **kwargs):
    native.genrule(
        name = name,
        srcs = srcs,
        local = 1,
        outs = [name + ".released"],
        cmd = "TMPDIR=/tmp gsutil cp $(SRCS) %s > $@; gsutil stat %s/`basename $(SRCS)`" % (gcs_bucket, gcs_bucket),
        **kwargs
        )