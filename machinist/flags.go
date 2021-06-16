package machinist

import "net"

type SharedFlags struct {
	Listener         net.Listener
	Insecure         bool
	ControlPlanePort int
	ControlPlaneHost string
}

type SharedFlagsProvider interface {
	MachinistFlags() *SharedFlags
}

type Modifier func(s SharedFlagsProvider) error

func WithListener(l net.Listener) Modifier {
	return func(s SharedFlagsProvider) error {
		s.MachinistFlags().Listener = l
		return nil
	}
}

func WithInsecure() Modifier {
	return func(s SharedFlagsProvider) error {
		s.MachinistFlags().Insecure = true
		return nil
	}
}
