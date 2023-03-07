module github.com/enfabrica/enkit

go 1.16

require (
	bazil.org/fuse v0.0.0-20200524192727-fb710f7dfd05
	cloud.google.com/go/datastore v1.1.0
	cloud.google.com/go/storage v1.27.0
	github.com/GoogleCloudPlatform/cloud-build-notifiers v0.0.0-20221005190102-4a3420d331aa // indirect
	github.com/KohlsTechnology/prometheus_bigquery_remote_storage_adapter v0.4.6 // indirect
	github.com/Masterminds/sprig/v3 v3.2.2 // indirect
	github.com/Microsoft/go-winio v0.4.16 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/bazelbuild/buildtools v0.0.0-20211007154642-8dd79e56e98e
	github.com/bazelbuild/rules_go v0.32.0
	github.com/cheggaaa/pb/v3 v3.0.5
	github.com/containerd/containerd v1.4.3 // indirect
	github.com/coreos/go-oidc v2.2.1+incompatible
	github.com/cybozu-go/aptutil v1.4.2-0.20200413001041-3f82d8384481
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/docker/docker v20.10.5+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.12.0
	github.com/fullstorydev/grpcurl v1.8.5 // indirect
	github.com/go-git/go-git/v5 v5.1.0
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/go-lintpack/lintpack v0.5.2 // indirect
	github.com/golang-collections/go-datastructures v0.0.0-20150211160725-59788d5eb259 // indirect
	github.com/golang/protobuf v1.5.2
	github.com/golang/snappy v0.0.4 // indirect
	github.com/golangci/errcheck v0.0.0-20181223084120-ef45e06d44b6 // indirect
	github.com/golangci/go-tools v0.0.0-20190318055746-e32c54105b7c // indirect
	github.com/golangci/goconst v0.0.0-20180610141641-041c5f2b40f3 // indirect
	github.com/golangci/gocyclo v0.0.0-20180528134321-2becd97e67ee // indirect
	github.com/golangci/golangci-lint v1.38.0 // indirect
	github.com/golangci/gosec v0.0.0-20190211064107-66fb7fc33547 // indirect
	github.com/golangci/ineffassign v0.0.0-20190609212857-42439a7714cc // indirect
	github.com/golangci/prealloc v0.0.0-20180630174525-215b22d4de21 // indirect
	github.com/google/cel-go v0.11.3 // indirect
	github.com/google/go-cmp v0.5.9
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/uuid v1.3.0
	github.com/googleapis/go-type-adapters v1.0.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.1.0 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/improbable-eng/grpc-web v0.13.0
	github.com/jackpal/gateway v1.0.7 // indirect
	github.com/itchyny/gojq v0.12.9 // indirect
	github.com/josephburnett/jd v1.6.1
	github.com/kataras/muxie v1.1.1
	github.com/kirsle/configdir v0.0.0-20170128060238-e45d2f54772f
	github.com/klauspost/compress v1.15.1 // indirect
	github.com/klauspost/cpuid v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/go-ps v1.0.0
	github.com/mitchellh/mapstructure v1.4.2
	github.com/pelletier/go-toml v1.8.1
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	github.com/pquerna/cachecontrol v0.0.0-20200921180117-858c6e7e6b7e // indirect
	github.com/prashantv/gostub v1.0.0
	github.com/prometheus/client_golang v1.12.1
	github.com/psanford/memfs v0.0.0-20210214183328-a001468d78ef
	github.com/rs/cors v1.7.0 // indirect
	github.com/shirou/gopsutil v0.0.0-20180427012116-c95755e4bcd7 // indirect
	github.com/shirou/w32 v0.0.0-20160930032740-bb4de0191aa4 // indirect
	github.com/soheilhy/cmux v0.1.4
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/tchap/zapext v1.0.0
	github.com/ulikunitz/xz v0.5.8
	github.com/valyala/quicktemplate v1.7.0 // indirect
	github.com/xenking/zipstream v1.0.1 // indirect
	github.com/xor-gate/ar v0.0.0-20170530204233-5c72ae81e2b7
	go.uber.org/zap v1.19.1
	golang.org/x/crypto v0.0.0-20210915214749-c084706c2272
	// BUG(INFRA-1801): Last version that supports go1.16 is golang.org/x/net v0.0.0-20211020060615-d418f374d309
	golang.org/x/net v0.7.0
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	golang.org/x/sync v0.0.0-20220601150217-0de741cfad7f // indirect
	// BUG(INFRA-1801): Last version that supports go1.16 is golang.org/x/sys v0.0.0-20220908164124-27713097b956
	golang.org/x/sys v0.0.0-20220908164124-27713097b956 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/tools v0.1.9 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/api v0.102.0
	google.golang.org/genproto v0.0.0-20221027153422-115e99e71e1c // indirect
	google.golang.org/grpc v1.50.1
	google.golang.org/protobuf v1.28.1
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	gopkg.in/yaml.v2 v2.4.0
	sourcegraph.com/sqs/pbtypes v0.0.0-20180604144634-d3ebe8f20ae4 // indirect
)
