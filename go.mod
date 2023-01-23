module github.com/enfabrica/enkit

go 1.16

require (
	cloud.google.com/go/bigquery v1.44.0
	cloud.google.com/go/datastore v1.10.0
	cloud.google.com/go/storage v1.27.0
	github.com/GoogleCloudPlatform/cloud-build-notifiers v0.0.0-20221005190102-4a3420d331aa
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/bazelbuild/buildtools v0.0.0-20211007154642-8dd79e56e98e
	github.com/bazelbuild/rules_go v0.32.0
	github.com/cheggaaa/pb/v3 v3.0.5
	github.com/coreos/go-oidc v2.2.1+incompatible
	github.com/cybozu-go/aptutil v1.4.2-0.20200413001041-3f82d8384481
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/docker/docker v20.10.5+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.12.0
	github.com/go-git/go-git/v5 v5.1.0
	github.com/golang/glog v1.0.0
	github.com/golang/protobuf v1.5.2
	github.com/google/cel-go v0.11.3 // indirect
	github.com/google/go-cmp v0.5.9
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/improbable-eng/grpc-web v0.13.0
	github.com/itchyny/gojq v0.12.9
	github.com/jackpal/gateway v1.0.7
	github.com/josephburnett/jd v1.6.1
	github.com/kataras/muxie v1.1.1
	github.com/kirsle/configdir v0.0.0-20170128060238-e45d2f54772f
	github.com/klauspost/compress v1.15.1 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/miekg/dns v1.1.43
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/go-ps v1.0.0
	github.com/mitchellh/mapstructure v1.4.2
	github.com/pelletier/go-toml v1.8.1
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	github.com/pquerna/cachecontrol v0.0.0-20200921180117-858c6e7e6b7e // indirect
	github.com/prashantv/gostub v1.0.0
	github.com/prometheus/client_golang v1.12.1
	github.com/psanford/memfs v0.0.0-20210214183328-a001468d78ef
	github.com/rs/cors v1.7.0
	github.com/sirupsen/logrus v1.8.1
	github.com/soheilhy/cmux v0.1.4
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.1
	github.com/tchap/zapext v1.0.0
	github.com/ulikunitz/xz v0.5.8
	github.com/xenking/zipstream v1.0.1
	github.com/xor-gate/ar v0.0.0-20170530204233-5c72ae81e2b7
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/goleak v1.1.12
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.1
	golang.org/x/crypto v0.0.0-20211108221036-ceb1ce70b4fa
	// BUG(INFRA-1801): Last version that supports go1.16 is golang.org/x/net v0.0.0-20211020060615-d418f374d309
	golang.org/x/net v0.5.0
	golang.org/x/oauth2 v0.4.0
	google.golang.org/api v0.103.0
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f
	google.golang.org/grpc v1.52.0
	google.golang.org/grpc/examples v0.0.0-20230123225046-4075ef07c5d5 // indirect
	google.golang.org/protobuf v1.28.1
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/client-go v0.20.6 // indirect
)
