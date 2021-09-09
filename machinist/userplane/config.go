package userplane

import (
	"fmt"
	"net"
	"time"
)

type Config struct {
	BindPort      string
	Port          int
	BindDns       string
	DnsPort       int
	StateFile     string
	StateWriteTTL time.Duration
	Lis           net.Listener
}

func (c *Config) Verify() error {
	if c.Lis == nil {
		l, err :=  net.Listen("tcp", fmt.Sprintf("%s:%d", c.BindPort, c.Port))
		if err != nil {
			return err
		}
		c.Lis = l
	}
	return nil
}

type ConfigMod func(c *Config) *Config

var (
	WithConfig = func(c *Config) ConfigMod {
		return func(c1 *Config) *Config {
			return c
		}
	}
	WithListener = func(l net.Listener) ConfigMod {
		return func(c *Config) *Config {
			c.Lis = l
			return c
		}
	}
	WithPort = func(p int) ConfigMod {
		return func(c *Config) *Config {
			c.Port = p
			return c
		}
	}
	WithDnsPort = func(dp int) ConfigMod {
		return func(c *Config) *Config {
			c.DnsPort = dp
			return c
		}
	}
)
