package machinist

import (
	"net"
)

type SharedFlags struct {
	Listener net.Listener
	Insecure bool
	Host     string
	Port     int
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

func WithHost(h string) Modifier {
	return func(s SharedFlagsProvider) error {
		s.MachinistFlags().Host = h
		return nil
	}
}
func WithPort(p int) Modifier {
	return func(s SharedFlagsProvider) error {
		s.MachinistFlags().Port = p
		return nil
	}
}
