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
	Root             *client.BaseFlags
}

type MachinistCommon interface {
	MachinistCommon() *Common
}
