package asset_service

import "sync/atomic"

type Metrics interface {
	OnFetchRequested()
	NumberOfRequestedFetches() uint64
	OnFetchStarted()
	NumberOfFetches() uint64
}

type metrics struct {
	fetchesRequested atomic.Uint64
	fetchesStarted   atomic.Uint64
}

func NewMetrics() Metrics {
	return &metrics{
		fetchesRequested: atomic.Uint64{},
		fetchesStarted:   atomic.Uint64{},
	}
}

func (m *metrics) OnFetchRequested() {
	m.fetchesRequested.Add(1)
}

func (m *metrics) NumberOfRequestedFetches() uint64 {
	return m.fetchesRequested.Load()
}

func (m *metrics) OnFetchStarted() {
	m.fetchesStarted.Add(1)
}

func (m *metrics) NumberOfFetches() uint64 {
	return m.fetchesStarted.Load()
}
