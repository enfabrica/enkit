module github.com/enfabrica/enkit

go 1.20

replace github.com/GoogleCloudPlatform/cloud-build-notifiers => github.com/minor-fixes/cloud-build-notifiers v0.0.0-20230424124639-02281bcdd3d5

// Required by buildbarn ecosystem
replace github.com/grpc-ecosystem/grpc-gateway/v2 => github.com/grpc-ecosystem/grpc-gateway/v2 v2.16.1

require (
	cloud.google.com/go/bigquery v1.56.0
	cloud.google.com/go/cloudbuild v1.14.2
	cloud.google.com/go/datastore v1.15.0
	cloud.google.com/go/pubsub v1.33.0
	cloud.google.com/go/storage v1.35.1
	github.com/bazelbuild/buildtools v0.0.0-20220208174414-fc5b9bb898c9
	github.com/bazelbuild/remote-apis v0.0.0-20230822133051-6c32c3b917cc
	github.com/bazelbuild/rules_go v0.32.0
	github.com/buildbarn/bb-remote-execution v0.0.0-20230905173453-70efb72857b0
	github.com/buildbarn/bb-storage v0.0.0-20231111202247-ece87ab6dc2a
	github.com/cheggaaa/pb/v3 v3.0.5
	github.com/coreos/go-oidc v2.2.1+incompatible
	github.com/cybozu-go/aptutil v1.4.2-0.20200413001041-3f82d8384481
	github.com/docker/docker v24.0.7+incompatible
	github.com/dustin/go-humanize v1.0.1
	github.com/fatih/color v1.16.0
	github.com/go-git/go-git/v5 v5.1.0
	github.com/golang/glog v1.1.2
	github.com/golang/protobuf v1.5.3
	github.com/google/go-cmp v0.6.0
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-jsonnet v0.20.1-0.20230626194039-fed90cd9cd73
	github.com/google/uuid v1.4.0
	github.com/gorilla/websocket v1.5.0
	github.com/hashicorp/nomad v1.6.3
	github.com/improbable-eng/grpc-web v0.13.0
	github.com/josephburnett/jd v1.6.1
	github.com/kataras/muxie v1.1.1
	github.com/kirsle/configdir v0.0.0-20170128060238-e45d2f54772f
	github.com/miekg/dns v1.1.50
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/go-ps v1.0.0
	github.com/mitchellh/mapstructure v1.5.0
	github.com/pelletier/go-toml v1.8.1
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	github.com/prashantv/gostub v1.0.0
	github.com/prometheus/client_golang v1.17.0
	github.com/psanford/memfs v0.0.0-20210214183328-a001468d78ef
	github.com/sirupsen/logrus v1.9.0
	github.com/soheilhy/cmux v0.1.5
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.4
	github.com/tchap/zapext v1.0.0
	github.com/ulikunitz/xz v0.5.10
	github.com/valyala/bytebufferpool v1.0.0
	github.com/valyala/quicktemplate v1.7.0
	github.com/xor-gate/ar v0.0.0-20170530204233-5c72ae81e2b7
	go.uber.org/goleak v1.2.1
	go.uber.org/zap v1.19.1
	golang.org/x/crypto v0.15.0
	golang.org/x/net v0.18.0
	golang.org/x/oauth2 v0.14.0
	google.golang.org/api v0.150.0
	google.golang.org/genproto/googleapis/bytestream v0.0.0-20231106174013-bbf56f31fb17
	google.golang.org/grpc v1.59.0
	google.golang.org/protobuf v1.31.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/GoogleCloudPlatform/cloud-build-notifiers v0.0.0-20230123211209-f695cd1064aa
	github.com/Masterminds/sprig/v3 v3.2.3
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/cel-go v0.14.0 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/itchyny/gojq v0.12.9
	github.com/jackpal/gateway v1.0.7
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/pquerna/cachecontrol v0.0.0-20200921180117-858c6e7e6b7e // indirect
	github.com/rs/cors v1.8.3
	github.com/xenking/zipstream v1.0.1
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/sys v0.14.0 // indirect
	golang.org/x/tools v0.15.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/genproto v0.0.0-20231030173426-d783a09b4405
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
)

require (
	cloud.google.com/go v0.110.9 // indirect
	cloud.google.com/go/compute v1.23.2 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/iam v1.1.4 // indirect
	cloud.google.com/go/longrunning v0.5.4 // indirect
	cloud.google.com/go/secretmanager v1.11.3 // indirect
	github.com/LK4D4/joincontext v0.0.0-20171026170139-1724345da6d5 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.0 // indirect
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230321174746-8dcc6526cfb1 // indirect
	github.com/apache/arrow/go/v12 v12.0.0 // indirect
	github.com/apache/thrift v0.18.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.0.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/flatbuffers v23.3.3+incompatible // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/googleapis/gax-go/v2 v2.12.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-hclog v1.5.0 // indirect
	github.com/hashicorp/go-msgpack v1.1.5 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-plugin v1.6.0 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/itchyny/timefmt-go v0.1.4 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20190725054713-01f96b0aa0cd // indirect
	github.com/klauspost/asmfmt v1.3.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/minio/asm2plan9s v0.0.0-20200509001527-cdd76441f9d8 // indirect
	github.com/minio/c2goasm v0.0.0-20190812172519-36a3d3bbc4f3 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-testing-interface v1.14.2-0.20210821155943-2d9075ca8770 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/oklog/run v1.1.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.17 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.11.1 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/xanzy/ssh-agent v0.2.1 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/exp v0.0.0-20231110203233-9a3e6036ecaa // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231030173426-d783a09b4405 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231106174013-bbf56f31fb17 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/client-go v0.27.1 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)
