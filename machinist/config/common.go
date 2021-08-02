package config

import (
	"github.com/enfabrica/enkit/lib/client"
	"net"
)

type Common struct {
	Listener         net.Listener
	Insecure         bool
	ControlPlanePort int
	ControlPlaneHost string
	MetricsPort      int
	EnableMetrics    bool
	Root             *client.BaseFlags
}

type MachinistCommon interface {
	MachinistCommon() *Common
}

type CommonModifier func(s MachinistCommon) error

func WithListener(l net.Listener) CommonModifier {
	return func(s MachinistCommon) error {
		s.MachinistCommon().Listener = l
		return nil
	}
}

func WithInsecure() CommonModifier {
	return func(s MachinistCommon) error {
		s.MachinistCommon().Insecure = true
		return nil
	}
}

func WithMetricsPort(p int) CommonModifier {
	return func(s MachinistCommon) error {
		s.MachinistCommon().MetricsPort = p
		return nil
	}
}

func WithEnableMetrics(e bool) CommonModifier {
	return func(s MachinistCommon) error {
		s.MachinistCommon().EnableMetrics = e
		return nil
	}
}

func DefaultCommonFlags() *Common {
	return &Common{
		Root: client.DefaultBaseFlags("", ""),
	}
}
