package asset_service

import (
	"context"
	"encoding/json"
	"github.com/buildbarn/bb-storage/pkg/clock"
	bbgrpc "github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/jmespath"
	"github.com/buildbarn/bb-storage/pkg/program"
	bbconfig "github.com/buildbarn/bb-storage/pkg/proto/configuration/grpc"
	jmespathconfig "github.com/buildbarn/bb-storage/pkg/proto/configuration/jmespath"
	"github.com/buildbarn/bb-storage/pkg/proto/configuration/tls"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/google/go-jsonnet"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type CacheConfig interface {
	ProxyAddress() *url.URL
	TlsConfig() *tls.ClientConfiguration
}

type Config interface {
	GrpcAddress() string

	CacheConfig() CacheConfig
	MetadataExtractor() bbgrpc.MetadataExtractor

	ParallelDownloads() int32
	SkipSchemes() map[string]bool
	SkipHosts() map[string]bool
	AccessLogger() *log.Logger
	ErrorLogger() *log.Logger
}

type cacheConfig struct {
	Address *url.URL                 `json:"address"`
	Tls     *tls.ClientConfiguration `json:"tls,omitempty"`
}

func (cc *cacheConfig) UnmarshalJSON(data []byte) error {
	type Doppelganger cacheConfig

	tmp := struct {
		Address string `json:"address"`
		*Doppelganger
	}{
		Doppelganger: (*Doppelganger)(cc),
	}

	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	address, err := url.Parse(tmp.Address)
	if err != nil {
		return err
	}

	cc.Address = address

	return nil
}

func (cc *cacheConfig) ProxyAddress() *url.URL {
	return cc.Address
}

func (cc *cacheConfig) TlsConfig() *tls.ClientConfiguration {
	return cc.Tls
}

type config struct {
	Server struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`
	Cache           *cacheConfig `json:"cache,omitempty"`
	AssetDownloader struct {
		QueueSize int32 `json:"queue_size"`
	} `json:"asset_downloader"`
	UrlFilter struct {
		SkipHosts []string `json:"skip_hosts"`
	} `json:"url_filter"`
	Metadata struct {
		AddMetadata                   []*bbconfig.ClientConfiguration_HeaderValues `json:"add_metadata,omitempty"`
		AddMetadataJmespathExpression *jmespathconfig.Expression                   `json:"add_metadata_jmespath_expression,omitempty"`
	} `json:"metadata"`
	metadataExtractor bbgrpc.MetadataExtractor
}

type metadataExtractor struct {
	jmespathMetadataExtractor bbgrpc.MetadataExtractor
	addMetadata               bbgrpc.MetadataHeaderValues
}

func (m *metadataExtractor) extractMetadata(ctx context.Context) (bbgrpc.MetadataHeaderValues, error) {
	extraMetadata := m.addMetadata

	if m.jmespathMetadataExtractor != nil {
		jmespathMetadata, err := m.jmespathMetadataExtractor(ctx)
		if err != nil {
			return nil, err
		}
		extraMetadata = append(extraMetadata, jmespathMetadata...)
	}

	return extraMetadata, nil
}

func (cfg *config) Init(group program.Group) error {
	var metadataHeaderValues bbgrpc.MetadataHeaderValues
	for _, entry := range cfg.Metadata.AddMetadata {
		metadataHeaderValues.Add(entry.Header, entry.Values)
	}

	var jmespathMetadataExtractor bbgrpc.MetadataExtractor
	if cfg.Metadata.AddMetadataJmespathExpression != nil {
		expr, err := jmespath.NewExpressionFromConfiguration(cfg.Metadata.AddMetadataJmespathExpression, group, clock.SystemClock)
		if err != nil {
			return util.StatusWrap(err, "Failed to compile JMESPath expression")
		}

		jmespathMetadataExtractor, err = bbgrpc.NewJMESPathMetadataExtractor(expr)
		if err != nil {
			return util.StatusWrap(err, "Failed to create JMESPath extractor")
		}
	}

	if len(metadataHeaderValues) > 0 || jmespathMetadataExtractor != nil {
		metadataExtractor := &metadataExtractor{
			jmespathMetadataExtractor: jmespathMetadataExtractor,
			addMetadata:               metadataHeaderValues,
		}
		cfg.metadataExtractor = metadataExtractor.extractMetadata
	}

	return nil
}

func (cfg *config) GrpcAddress() string {
	return net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
}

func (cfg *config) CacheConfig() CacheConfig {
	return cfg.Cache
}

func (cfg *config) ParallelDownloads() int32 {
	if cfg.AssetDownloader.QueueSize == 0 {
		return 1024
	}
	return cfg.AssetDownloader.QueueSize
}

func (cfg *config) SkipSchemes() map[string]bool {
	return map[string]bool{"file": true}
}

func (cfg *config) SkipHosts() map[string]bool {
	result := map[string]bool{}
	for _, host := range cfg.UrlFilter.SkipHosts {
		result[host] = true
	}
	return result
}

func (cfg *config) AccessLogger() *log.Logger {
	return log.New(os.Stdout, "", log.LstdFlags)
}

func (cfg *config) ErrorLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func (cfg *config) MetadataExtractor() bbgrpc.MetadataExtractor {
	return cfg.metadataExtractor
}

func injectEnv(vm *jsonnet.VM) error {
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			return status.Errorf(codes.InvalidArgument, "Invalid environment variable: %#v", env)
		}
		vm.ExtVar(parts[0], parts[1])
	}
	return nil
}

func NewConfigFromStr(data string, group program.Group) (Config, error) {
	// Create config structure
	config := &config{}

	vm := jsonnet.MakeVM()
	err := injectEnv(vm)
	if err != nil {
		return nil, err
	}

	jsonStr, err := vm.EvaluateAnonymousSnippet("config.jsonnet", data)
	if err != nil {
		return nil, err
	}

	// Init new YAML decode
	err = json.Unmarshal([]byte(jsonStr), config)

	// Start YAML decoding from file
	if err != nil {
		return nil, err
	}

	err = config.Init(group)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func NewConfigFromPath(configPath string, group program.Group) (Config, error) {
	// Create config structure
	config := &config{}

	vm := jsonnet.MakeVM()
	err := injectEnv(vm)
	if err != nil {
		return nil, err
	}

	jsonStr, err := vm.EvaluateFile(configPath)
	if err != nil {
		return nil, err
	}

	// Init new YAML decode
	err = json.Unmarshal([]byte(jsonStr), config)

	// Start YAML decoding from file
	if err != nil {
		return nil, err
	}

	err = config.Init(group)
	if err != nil {
		return nil, err
	}

	return config, nil
}
