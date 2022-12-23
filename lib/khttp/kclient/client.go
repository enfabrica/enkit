package kclient

import (
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/khttp/ktls"
	"github.com/enfabrica/enkit/lib/khttp/ktransport"
	"net/http"
)

type Modifier func(c *http.Client) error

type Modifiers []Modifier

type Flags struct {
	*ktransport.RTFlags
}

func DefaultFlags() *Flags {
	flags := &Flags{
		RTFlags: ktransport.DefaultRTFlags(),
	}

	return flags
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	fl.RTFlags.Register(set, prefix)
	return fl
}

// FromFlags applies the configurations defined in the Flags object.
//
// Additional transport modifiers can be supplied here that will be applied
// together with the flags. Use the mods here whenever it is necessary to
// supply http2.Transport parameters for a potentially http.Transport, as
// those configs cannot be applied incrementally due to constraints in the
// golang API.
func FromFlags(fl *Flags, mods ...ktransport.RTModifier) Modifier {
	return func(c *http.Client) error {
		if fl == nil {
			return nil
		}

		transport, err := ktransport.DefaultOrNew(fl.RTFlags, mods...)
		if err != nil {
			return err
		}

		c.Transport = transport
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

// WithJar overrides the default client http.CookieJar with the specified one.
//
// A CookieJar will automatically store and retrieve cookies based on the remote
// path and domain name retrieved, and generally implement the logic that browsers
// use to protect and provide cookies.
func WithJar(jar http.CookieJar) Modifier {
	return func(c *http.Client) error {
		c.Jar = jar
		return nil
	}
}

// WithTransport replaces the transport used by the client.
func WithTransport(rt http.RoundTripper) Modifier {
	return func(c *http.Client) error {
		c.Transport = rt
		return nil
	}
}

// WithTransportOptions applies the supplied options to the transport.
func WithTransportOptions(mods ...ktransport.RTModifier) Modifier {
	return func(c *http.Client) error {
		if c.Transport == nil {
			c.Transport = &http.Transport{}
		}
		return ktransport.RTModifiers(mods).Apply(c.Transport)
	}
}

// WithRedirectHandler configures a custom redirect handler in the client (see CheckRedirect in http.Client).
func WithRedirectHandler(handler func(req *http.Request, via []*http.Request) error) Modifier {
	return func(c *http.Client) error {
		c.CheckRedirect = handler
		return nil
	}
}

// WithDisabledRedirects disables redirects in the client.
func WithDisabledRedirects() Modifier {
	return WithRedirectHandler(func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	})
}

// WithInsecureCertificates configures the transport to allow insecure certs.
func WithInsecureCertificates() Modifier {
	return func(c *http.Client) error {
		return WithTransportOptions(
			ktransport.WithRTTLSOptions(ktls.WithInsecureCertificates()),
		)(c)
	}
}
