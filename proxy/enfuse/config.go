package enfuse

import (
	"crypto/tls"
	"crypto/x509"
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
	WithCertPool = func(certPool *x509.CertPool) ConnectMod {
		return func(c *ConnectConfig) {
			c.ClientCredentials = certPool
		}
	}
)

type ClientEncryptionInfo struct {
	Pool        *x509.CertPool
	RootPool    *x509.CertPool
	Certificate tls.Certificate
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
