package kserver

import (
	"crypto/tls"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp/ktls"
	"golang.org/x/net/http2"
	"net/http"
	"time"
)

type Modifier func(s *http.Server) error

type Modifiers []Modifier

// Apply applies the set of modifiers to the specified config.
func (mods Modifiers) Apply(s *http.Server) error {
	for _, m := range mods {
		if err := m(s); err != nil {
			return err
		}
	}
	return nil
}

type Flags struct {
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

func DefaultFlags() *Flags {
	return &Flags{
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
	}
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.DurationVar(&fl.ReadTimeout, prefix+"http-read-timeout", fl.ReadTimeout,
		"ReadTimeout to configure in the server - see https://pkg.go.dev/net/http#Server")
	set.DurationVar(&fl.ReadHeaderTimeout, prefix+"http-read-header-timeout", fl.ReadHeaderTimeout,
		"ReadHeaderTimeout to configure in the server - see https://pkg.go.dev/net/http#Server")
	set.DurationVar(&fl.WriteTimeout, prefix+"http-write-timeout", fl.WriteTimeout,
		"WriteTimeout to configure in the server - see https://pkg.go.dev/net/http#Server")
	set.DurationVar(&fl.IdleTimeout, prefix+"http-idle-timeout", fl.IdleTimeout,
		"IdleTimeout to configure in the server - see https://pkg.go.dev/net/http#Server")
	set.IntVar(&fl.MaxHeaderBytes, prefix+"http-max-header-bytes", fl.MaxHeaderBytes,
		"MaxHeaderBytes to configure in the server - see https://pkg.go.dev/net/http#Server")

	return fl
}

type TLSFlags struct {
	TLS *ktls.Flags
	*Flags
}

func DefaultTLSFlags() *TLSFlags {
	return &TLSFlags{
		TLS:   ktls.DefaultFlags(),
		Flags: DefaultFlags(),
	}
}

func (fl *TLSFlags) Register(set kflags.FlagSet, prefix string) *TLSFlags {
	fl.TLS.Register(set, prefix)
	fl.Flags.Register(set, prefix)

	return fl
}

func FromFlags(fl *Flags) Modifier {
	return func(s *http.Server) error {
		return Modifiers{
			WithReadTimeout(fl.ReadTimeout),
			WithReadHeaderTimeout(fl.ReadHeaderTimeout),
			WithWriteTimeout(fl.WriteTimeout),
			WithIdleTimeout(fl.IdleTimeout),
			WithMaxHeaderBytes(fl.MaxHeaderBytes),
		}.Apply(s)
	}
}

func FromTLSFlags(fl *TLSFlags) Modifier {
	return func(s *http.Server) error {
		if err := WithTLSOptions(ktls.FromFlags(fl.TLS))(s); err != nil {
			return err
		}

		return FromFlags(fl.Flags)(s)
	}
}

// WithReadTimeout configures the ReadTimeout in an http.Server.
func WithReadTimeout(timeout time.Duration) Modifier {
	return func(s *http.Server) error {
		s.ReadTimeout = timeout
		return nil
	}
}

// WithReadHeaderTimeout configures the ReadHeaderTimeout in an http.Server.
func WithReadHeaderTimeout(timeout time.Duration) Modifier {
	return func(s *http.Server) error {
		s.ReadHeaderTimeout = timeout
		return nil
	}
}

// WithWriteTimeout configures the WriteTimeout in an http.Server.
func WithWriteTimeout(timeout time.Duration) Modifier {
	return func(s *http.Server) error {
		s.WriteTimeout = timeout
		return nil
	}
}

// WithIdleTimeout configures the IdleTimeout in an http.Server.
func WithIdleTimeout(timeout time.Duration) Modifier {
	return func(s *http.Server) error {
		s.IdleTimeout = timeout
		return nil
	}
}

// WithMaxHeaderBytes configures the MaxHeaderSize in an http.Server.
func WithMaxHeaderBytes(size int) Modifier {
	return func(s *http.Server) error {
		s.MaxHeaderBytes = size
		return nil
	}
}

// WithTLSOptions applies the tls modifiers to the server configuration.
func WithTLSOptions(mods ...ktls.Modifier) Modifier {
	return func(s *http.Server) error {
		config := s.TLSConfig
		if config == nil {
			config = &tls.Config{}
		} else {
			config = config.Clone()
		}

		if err := ktls.Modifiers(mods).Apply(config); err != nil {
			return err
		}

		s.TLSConfig = config
		return nil
	}
}

// WithTLSConfig adds a tls server configuration.
func WithTLSConfig(config *tls.Config) Modifier {
	return func(s *http.Server) error {
		s.TLSConfig = config
		return nil
	}
}

type Modifier2 func(s *http2.Server) error

type Modifiers2 []Modifier2

// Apply applies the set of modifiers to the specified config.
func (mods Modifiers2) Apply(s *http2.Server) error {
	for _, m := range mods {
		if err := m(s); err != nil {
			return err
		}
	}
	return nil
}

// TODO: add modifiers for http2 server.
