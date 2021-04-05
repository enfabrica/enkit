package kdns

import (
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/logger/klog"
	"github.com/miekg/dns"
	"net"
)

func NewDNS(mods ...DNSModifier) (*DnsServer, error) {
	s := &DnsServer{
		routeMap: make(map[string][]string),
		Logger:   &logger.NilLogger{},
	}
	for _, mod := range mods {
		if err := mod(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

type DNSModifier func(s *DnsServer) error

func WithLogger(l *klog.Logger) DNSModifier {
	return func(s *DnsServer) error {
		s.Logger = l
		return nil
	}
}

func WithPort(p int) DNSModifier {
	return func(s *DnsServer) error {
		s.Port = p
		return nil
	}
}

func WithDomains(domains []string) DNSModifier {
	return func(s *DnsServer) error {
		s.domains = domains
		for _, d :=  range domains {
			s.routeMap[dns.Fqdn(d)] = &BaseRecord{}
		}
		return nil
	}
}

func WithListener(l net.Listener) DNSModifier {
	return func(s *DnsServer) error {
		s.Listener = l
		return nil
	}
}

func WithHost(ip string) DNSModifier {
	return func(s *DnsServer) error {
		s.host = ip
		return nil
	}
}

