package asset_service

import "net/url"

type UrlFilter interface {
	CanProceed(url *url.URL) bool
}

type urlFilter struct {
	skipSchemes map[string]bool
	skipHosts   map[string]bool
}

func NewUrlFilter(config Config) UrlFilter {
	return &urlFilter{
		skipSchemes: config.SkipSchemes(),
		skipHosts:   config.SkipHosts(),
	}
}

func (uf *urlFilter) CanProceed(url *url.URL) bool {
	if _, exists := uf.skipSchemes[url.Scheme]; exists {
		return false
	}
	if _, exists := uf.skipHosts[url.Host]; exists {
		return false
	}
	return true
}
