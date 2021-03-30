package machinist

import "net"

type SharedFlags struct {
	Listener net.Listener
	Insecure bool
}

type SharedFlagsProvider interface {
	Flags() *SharedFlags
}

type Modifier func(s SharedFlagsProvider) error

func WithListener(l net.Listener) Modifier {
	return func(s SharedFlagsProvider) error {
		s.Flags().Listener = l
		return nil
	}
}

func WithInsecure() Modifier {
	return func(s SharedFlagsProvider) error {
		s.Flags().Insecure = true
		return nil
	}
}
