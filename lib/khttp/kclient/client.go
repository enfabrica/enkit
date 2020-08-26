package kclient

import (
	"crypto/tls"
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"net/http"
	"time"
)

type Modifier func(c *http.Client) error

type Modifiers []Modifier

type Flags struct {
	ExpectContinueTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	IdleConnTimeout       time.Duration
	MaxIdleConns          int

	ForceAttemptHTTP2    bool
	InsecureCertificates bool
}

func DefaultFlags() *Flags {
	flags := &Flags{}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		flags.ExpectContinueTimeout = transport.ExpectContinueTimeout
		flags.TLSHandshakeTimeout = transport.TLSHandshakeTimeout
		flags.IdleConnTimeout = transport.IdleConnTimeout
		flags.MaxIdleConns = transport.MaxIdleConns
		flags.ForceAttemptHTTP2 = transport.ForceAttemptHTTP2
	}

	return flags
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.DurationVar(&fl.ExpectContinueTimeout, prefix+"http-expect-continue-timeout", fl.ExpectContinueTimeout, "How long to wait for a continue in a persistent http connection")
	set.DurationVar(&fl.TLSHandshakeTimeout, prefix+"http-tls-handshake-timeout", fl.TLSHandshakeTimeout, "How long to wait for the TLS Handshke to complete")
	set.DurationVar(&fl.IdleConnTimeout, prefix+"http-idle-conn-timeout", fl.IdleConnTimeout, "How long to keep a connection open before closing it")
	set.IntVar(&fl.MaxIdleConns, prefix+"http-max-idle-conns", fl.MaxIdleConns, "How many idle connections to keep at most")
	set.BoolVar(&fl.ForceAttemptHTTP2, prefix+"http-attempt-http2", fl.ForceAttemptHTTP2, "Try using HTTP2, fallback to HTTP1 if that does not work")
	set.BoolVar(&fl.InsecureCertificates, prefix+"http-insecure-certificates", fl.InsecureCertificates, "Allow insecure certificates from the server")
	return fl
}

func (fl *Flags) Matches(transport *http.Transport) bool {
	if transport.ExpectContinueTimeout != fl.ExpectContinueTimeout || transport.TLSHandshakeTimeout != fl.TLSHandshakeTimeout || transport.IdleConnTimeout != fl.IdleConnTimeout {
		return false
	}
	if transport.MaxIdleConns != fl.MaxIdleConns || transport.ForceAttemptHTTP2 != fl.ForceAttemptHTTP2 {
		return false
	}

	config := transport.TLSClientConfig
	if (config == nil && fl.InsecureCertificates) || (config != nil && config.InsecureSkipVerify != fl.InsecureCertificates) {
		return false
	}
	return true
}

func FromFlags(fl *Flags) Modifier {
	return func(c *http.Client) error {
		if fl == nil {
			return nil
		}

		transport, ok := c.Transport.(*http.Transport)
		if c.Transport == nil {
			transport, ok = http.DefaultTransport.(*http.Transport)
		}
		if !ok {
			return fmt.Errorf("cannot apply flags on non-http transport %#v", transport)
		}

		// Either the default transport or configured transport is already configured correctly. Nothing to do here.
		if fl.Matches(transport) {
			return nil
		}

		// Need to change the transport parameters. If it's a default transport, we need to create a new one.
		if c.Transport == nil {
			transport = &http.Transport{}
			c.Transport = transport
		}

		transport.ExpectContinueTimeout = fl.ExpectContinueTimeout
		transport.TLSHandshakeTimeout = fl.TLSHandshakeTimeout
		transport.IdleConnTimeout = fl.IdleConnTimeout
		transport.MaxIdleConns = fl.MaxIdleConns
		transport.ForceAttemptHTTP2 = fl.ForceAttemptHTTP2
		if fl.InsecureCertificates {
			return WithInsecureCertificates()(c)
		}

		return nil
	}
}

func (cg Modifiers) Apply(base *http.Client) error {
	for _, cm := range cg {
		if err := cm(base); err != nil {
			return err
		}
	}
	return nil
}

func transport(c *http.Client) (*http.Transport, error) {
	transport, ok := c.Transport.(*http.Transport)
	if c.Transport == nil {
		transport = &http.Transport{}
		c.Transport = transport
		return transport, nil
	}

	if !ok {
		return nil, fmt.Errorf("http client uses unknown transport - WithHttpInsecureCertificates cannot be enabled")
	}
	return transport, nil
}

func WithExpectContinueTimeout(timeout time.Duration) Modifier {
	return func(c *http.Client) error {
		transport, err := transport(c)
		if err != nil {
			return err
		}

		transport.ExpectContinueTimeout = timeout
		return nil
	}
}

func WithTLSHandshakeTimeout(timeout time.Duration) Modifier {
	return func(c *http.Client) error {
		transport, err := transport(c)
		if err != nil {
			return err
		}

		transport.TLSHandshakeTimeout = timeout
		return nil
	}
}

func WithIdleConnTimeout(timeout time.Duration) Modifier {
	return func(c *http.Client) error {
		transport, err := transport(c)
		if err != nil {
			return err
		}

		transport.IdleConnTimeout = timeout
		return nil
	}
}

func WithMaxIdleConns(value int) Modifier {
	return func(c *http.Client) error {
		transport, err := transport(c)
		if err != nil {
			return err
		}

		transport.MaxIdleConns = value
		return nil
	}
}

func WithForceAttemptHTTP2(value bool) Modifier {
	return func(c *http.Client) error {
		transport, err := transport(c)
		if err != nil {
			return err
		}

		transport.ForceAttemptHTTP2 = value
		return nil
	}
}

func WithTransport(rt http.RoundTripper) Modifier {
	return func(c *http.Client) error {
		c.Transport = rt
		return nil
	}
}

func WithInsecureCertificates() Modifier {
	return func(c *http.Client) error {
		transport, err := transport(c)
		if err != nil {
			return err
		}

		config := transport.TLSClientConfig
		if config == nil {
			config = &tls.Config{}
			transport.TLSClientConfig = config
		}

		config.InsecureSkipVerify = true
		return nil
	}
}
