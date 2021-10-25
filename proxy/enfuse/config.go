package enfuse

import (
	"net"
)

type ConnectConfig struct {
		Port int
		Url  string
		L    net.Listener
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

type (
	ServerConfig struct {
		*ConnectConfig
		Dir string
	}
	ServerConfigMod = func(sc *ServerConfig)
)

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
)

func NewServerConfig(mods ...ServerConfigMod) *ServerConfig {
	sc := &ServerConfig{
		ConnectConfig: &ConnectConfig{},
	}
	for _, m := range mods {
		m(sc)
	}
	return sc
}
