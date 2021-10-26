package enfuse

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/enfabrica/enkit/lib/srand"
	"math/rand"
	"net"
)

type ConnectConfig struct {
	Port              int
	Url               string
	L                 net.Listener
	ClientCredentials *x509.CertPool
	RootCAs           *x509.CertPool
	Certificate       tls.Certificate
	ServerName        string
	DnsNames          []string
	IpAddresses       []net.IP
}

type ConnectMod func(c *ConnectConfig)

var (
	WithPort = func(p int) ConnectMod {
		return func(c *ConnectConfig) {
			c.Port = p
		}
	}
	WithInterface = func(u string) ConnectMod {
		return func(c *ConnectConfig) {
			c.Url = u
		}
	}
	WithListener = func(l net.Listener) ConnectMod {
		return func(c *ConnectConfig) {
			c.L = l
		}
	}
	WithConnectConfig = func(c1 *ConnectConfig) ConnectMod {
		return func(c *ConnectConfig) {
			*c = *c1
		}
	}
)

// ApplyClientEncryptionInfo will add in the ClientEncryptionInfo into the right parameters in the connect config.
// if the certs are unwell, it will be rejected
func (cc *ConnectConfig) ApplyClientEncryptionInfo(cei *ClientEncryptionInfo) error {
	rootPool := x509.NewCertPool()
	if ok := rootPool.AppendCertsFromPEM(cei.CaPublicPem); !ok {
		return errors.New("CA certificate is invalid")
	}
	cc.RootCAs = rootPool
	interPool := x509.NewCertPool()
	if ok := interPool.AppendCertsFromPEM(cei.IntermediateCertPem); !ok {
		return errors.New("DCA certificate is invalid")
	}
	cc.ClientCredentials = interPool
	cer, err := tls.X509KeyPair(cei.ClientCertPem, cei.ClientPk)
	if err != nil {
		return err
	}
	cc.Certificate = cer
	return nil
}

// ClientEncryptionInfo contains the information necessary to connect to a server via mTLS. In this model, the DCA is
// held on the same server that has the Root CA. DCA revocation is still todo.
// It is designed to be easily serialized and deserialized or otherwise translated.
type ClientEncryptionInfo struct {
	CaPublicPem         []byte
	IntermediateCertPem []byte
	ClientPk            []byte
	ClientCertPem       []byte
}

type ServerConfig struct {
	*ConnectConfig
	Dir            string
	Rng            *rand.Rand
	ClientInfoChan chan *ClientEncryptionInfo
}

type ServerConfigMod = func(sc *ServerConfig)

var (
	WithConnectMods = func(c ...ConnectMod) ServerConfigMod {
		return func(sc *ServerConfig) {
			for _, m := range c {
				m(sc.ConnectConfig)
			}
		}
	}
	WithDir = func(d string) ServerConfigMod {
		return func(sc *ServerConfig) {
			sc.Dir = d
		}
	}
	WithEncryption = func(c chan *ClientEncryptionInfo) ServerConfigMod {
		return func(sc *ServerConfig) {
			sc.ClientInfoChan = c
		}
	}
)

func NewServerConfig(mods ...ServerConfigMod) *ServerConfig {
	rng := rand.New(srand.Source)
	sc := &ServerConfig{
		ConnectConfig: &ConnectConfig{},
		Rng:           rng,
	}
	for _, m := range mods {
		m(sc)
	}
	return sc
}
