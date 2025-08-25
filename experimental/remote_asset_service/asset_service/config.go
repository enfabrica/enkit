package asset_service

import (
	"gopkg.in/yaml.v2"
	"log"
	"net"
	"os"
	"strconv"
)

type Config interface {
	GrpcAddress() string
	ProxyAddress() string
	ParallelDownloads() int32
	SkipSchemes() map[string]bool
	SkipHosts() map[string]bool
	AccessLogger() *log.Logger
	ErrorLogger() *log.Logger
}

type config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Cache struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"cache"`
	AssetDownloader struct {
		QueueSize int32 `yaml:"queue_size"`
	} `yaml:"asset_downloader"`
	UrlFilter struct {
		SkipHosts []string `yaml:"skip_hosts"`
	} `yaml:"url_filter"`
}

func (cfg *config) GrpcAddress() string {
	return net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
}

func (cfg *config) ProxyAddress() string {
	return net.JoinHostPort(cfg.Cache.Host, strconv.Itoa(cfg.Cache.Port))
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

func NewConfigFromData(data []byte) (Config, error) {
	// Create config structure
	config := &config{}

	// Init new YAML decode
	err := yaml.Unmarshal(data, config)

	// Start YAML decoding from file
	if err != nil {
		return nil, err
	}

	return config, nil
}

func NewConfigFromPath(configPath string) (Config, error) {
	// Create config structure
	config := &config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}
