package asset_service

import (
	"encoding/json"
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

type Config interface {
	GrpcAddress() string
	ProxyAddress() *url.URL
	ProxyHeaders() map[string]string
	ParallelDownloads() int32
	SkipSchemes() map[string]bool
	SkipHosts() map[string]bool
	AccessLogger() *log.Logger
	ErrorLogger() *log.Logger
}

type cacheConfig struct {
	Address *url.URL          `json:"address"`
	Headers map[string]string `json:"headers"`
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

type config struct {
	Server struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`
	Cache           cacheConfig `json:"cache"`
	AssetDownloader struct {
		QueueSize int32 `json:"queue_size"`
	} `json:"asset_downloader"`
	UrlFilter struct {
		SkipHosts []string `json:"skip_hosts"`
	} `json:"url_filter"`
}

func (cfg *config) GrpcAddress() string {
	return net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
}

func (cfg *config) ProxyAddress() *url.URL {
	return cfg.Cache.Address
}

func (cfg *config) ProxyHeaders() map[string]string {
	return cfg.Cache.Headers
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

func NewConfigFromStr(data string) (Config, error) {
	// Create config structure
	config := &config{}

	vm := jsonnet.MakeVM()
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid environment variable: %#v", env)
		}
		vm.ExtVar(parts[0], parts[1])
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

	return config, nil
}

func NewConfigFromPath(configPath string) (Config, error) {
	// Create config structure
	config := &config{}

	vm := jsonnet.MakeVM()
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

	return config, nil
}
