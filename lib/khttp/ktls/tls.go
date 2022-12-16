// Package ktls provides modifiers to create and work with tls.Config objects.
//
// You can pass those modifiers to other functions in the khttp library, or use
// them to create a new tls.Config object via NewConfig.
//
// For example, to create a tls.Config object using a speific file as root CA,
// you can invoke:
//
//	tlsConfig, err := ktls.NewConfig(ktls.WithRootCAFile("/etc/root.crt"))
package ktls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"os"
)

type Modifier func(c *tls.Config) error

type Modifiers []Modifier

// Apply applies the set of modifiers to the specified config.
func (mods Modifiers) Apply(c *tls.Config) error {
	for _, m := range mods {
		if err := m(c); err != nil {
			return err
		}
	}
	return nil
}

// NewConfig creates a new tls config with the specified modifiers.
func NewConfig(mods ...Modifier) (*tls.Config, error) {
	config := &tls.Config{}
	if err := Modifiers(mods).Apply(config); err != nil {
		return nil, err
	}
	return config, nil
}

type Flags struct {
	InsecureSkipVerify bool
	DisableSystemCA    bool

	RootCA []byte

	CertData []byte
	CertKey  []byte
}

func DefaultFlags() *Flags {
	return &Flags{}
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.BoolVar(&fl.InsecureSkipVerify, prefix+"tls-insecure-skip-verify",
		fl.InsecureSkipVerify, "If set to true, disable SSL certificate verificaiton - any certificate is accepted")
	set.BoolVar(&fl.DisableSystemCA, prefix+"tls-disable-system-ca",
		fl.DisableSystemCA, "If set to true, system certificates will not be used/considered valid - all connections "+
			"will be rejected unless --tls-root-ca or --tls-insecure-skip-verify is used")

	set.ByteFileVar(&fl.RootCA, prefix+"tls-root-ca", "",
		"Path to a PEM encoded root certificate - peer certificates signed by this CA will be considered valid")

	set.ByteFileVar(&fl.CertData, prefix+"tls-cert-data", "",
		"Path to a certificate (.crt) - certificate presented to the peer to prove this code's identity")
	set.ByteFileVar(&fl.CertKey, prefix+"tls-cert-key", "",
		"Path to a certificate key (.key) - key to the certificate presented with --tls-cert-data")
	return fl
}

func FromFlags(fl *Flags) Modifier {
	return func (c *tls.Config) error {
		if fl.DisableSystemCA && !fl.InsecureSkipVerify && len(fl.RootCA) <= 0 {
			return kflags.NewUsageErrorf("--tls-disable-system-ca requires setting --tls-insecure-skip-verify or --tls-root-ca, otherwise TLS will always fail")
		}
		if len(fl.CertKey) > 0 && len(fl.CertData) <= 0 {
			return kflags.NewUsageErrorf("Specifying a TLS cert key requires specifying a TLS certificate as well (--tls-cert-key and --tls-cert-data)")
		}

		mods := []Modifier{}
		if fl.InsecureSkipVerify {
			mods = append(mods, WithInsecureCertificates())
		}
		if fl.DisableSystemCA {
			mods = append(mods, WithDisabledSystemRootCAs())
		}

		if len(fl.RootCA) > 0 {
			if !fl.DisableSystemCA {
				mods = append(mods, WithSystemRootCAs())
			}

			mods = append(mods, WithRootCAPEM(fl.RootCA))
		}

		if len(fl.CertData) > 0 || len(fl.CertKey) > 0 {
			mods = append(mods, WithCert(fl.CertData, fl.CertKey))
		}

		if err := Modifiers(mods).Apply(c); err != nil {
			return kflags.NewUsageErrorf("invalid --tls-... flags: %w", err)
		}
		return nil
	}
}

// WithInsecureCertificates skips CA certificate verification.
//
// See documentation on tls.Config InsecureSkipVerify.
func WithInsecureCertificates() Modifier {
	return func(c *tls.Config) error {
		c.InsecureSkipVerify = true
		return nil
	}
}

// DefaultRootCAsPool returns the default pool to use when nil.
//
// When a certificate is added to a tls.Config, a certificate
// pool needs to be either created or reused. This function
// returns the CertPool to use in a given tls.Config.
//
// By default, it create a new empty pool if none is specified
// in the tls.Config.
var DefaultRootCAsPool = func(c *tls.Config) *x509.CertPool {
	pool := c.RootCAs
	if pool == nil {
		pool = x509.NewCertPool()
		c.RootCAs = pool
	}

	return pool
}

// WithSystemRootCAs configures tls to explicitly accept the OS Root CAs.
//
// By default, the golang net/http libraries will accept all Root CAs
// configured in the operating system whenever tls.Config.RootCAs is
// left uninitialized - left to nil.
//
// As soon as one or more CAs are added explicitly to RootCAs, only
// those certificates will be considered valid.
//
// By invoking WithSystemRootCAs, the tls.Config is set to use a copy
// of the OS provided set of valid root CAs, allowing functions like
// WithRootCA to allow *additional* certificates to be considered
// valid.
//
// The order of functions is important: when WithSystemRootCAs is
// invoked, it replaces the current list of certificates.
func WithSystemRootCAs() Modifier {
	return func(c *tls.Config) error {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return err
		}

		c.RootCAs = pool
		return nil
	}
}

// WithDisabledSystemRootCAs configures tls to explicitly ignore the OS Root CAs.
//
// It initializes the tls.Config object to have an empty set of RootCAs.
func WithDisabledSystemRootCAs() Modifier {
	return func(c *tls.Config) error {
		c.RootCAs = x509.NewCertPool()
		return nil
	}
}

// WithRootCA adds the certificate as a valid Certification Authority.
//
// This is normally used in client code to accept server certificates
// signed by a normally untrusted authority (eg, a corp CA).
//
// WithRootCA appends the certificate to the pool defined for the
// tls.Config. On an unitialized tls.Config{} object, the resulting
// pool will only allow certificates specified with WithRootCA.
// No other certificate will be allowed.
//
// If you intend for CAs configured on your operating system to also
// be allowed, make sure to first invoke WithSystemRootCAs(), to
// initialize the pool with the system CAs.
//
// The golang libraries default to the System Root CAs if the set of
// certificates in tls.Config is nil.
func WithRootCA(cert *x509.Certificate) Modifier {
	return func(c *tls.Config) error {
		DefaultRootCAsPool(c).AddCert(cert)
		return nil
	}
}

// WithRootCAPEM adds a PEM encoded certificate.
//
// Just like WithRootCA, except it expects a byte array containing
// a PEM encoded certificate.
func WithRootCAPEM(cert []byte) Modifier {
	return func(c *tls.Config) error {
		if !DefaultRootCAsPool(c).AppendCertsFromPEM(cert) {
			return fmt.Errorf("failed to add certificate - invalid data?")
		}
		return nil
	}
}

// WithRootCAFile adds a PEM encoded certificate loaded from a file.
//
// Just like WithRootCA, except it expects the path of a file to
// be passed as an argument.
func WithRootCAFile(path string) Modifier {
	return func(c *tls.Config) error {
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if err := WithRootCAPEM(bytes)(c); err != nil {
			return fmt.Errorf("processing certificate in %s - %w", path, err)
		}
		return nil
	}
}

// WithCert adds a certificate to present to the peer at connection time.
//
// The certificate is presented when connecting to a peer.
// The peer can then decide to accept or reject the certificate.
//
// Multiple certificates are supported, this adds an additional certificate.
func WithCert(certf, keyf []byte) Modifier {
	return func(c *tls.Config) error {
		cert, err := tls.X509KeyPair(certf, keyf)
		if err != nil {
			return err
		}

		c.Certificates = append(c.Certificates, cert)
		return nil
	}
}

// WithCertFile adds a certificate from files.
//
// Just like WithCert, but loads the certificates from a file.
//
// The cert file is typically a file with .crt extension, while the key file
// typically as a .key extension.
func WithCertFile(certf, keyf string) Modifier {
	return func(c *tls.Config) error {
		cert, err := tls.LoadX509KeyPair(certf, keyf)
		if err != nil {
			return err
		}

		c.Certificates = append(c.Certificates, cert)
		return nil
	}
}

// WithGetCertificate configures a GetCertificate callback on the TLS config.
func WithGetCertificate(getter func(*tls.ClientHelloInfo) (*tls.Certificate, error)) Modifier {
	return func(c *tls.Config) error {
		c.GetCertificate = getter
		return nil
	}
}

// WithServerName configures the server name presented by the client or server.
func WithName(servername string) Modifier {
	return func(c *tls.Config) error {
		c.ServerName = servername
		return nil
	}
}
