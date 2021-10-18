package kdns

import (
	"github.com/enfabrica/enkit/lib/logger"
	"log"
	"net"
)

func NewDNS(mods ...DNSModifier) (*DnsServer, error) {
	s := &DnsServer{
		readOnlyChan: make(chan struct {
			Return chan *RecordController
			Origin string
		}),
		newOrExistingChan: make(chan struct {
			Return chan *RecordController
			Origin string
		}),
		Logger:          &logger.DefaultLogger{Printer: log.Printf},
		shutdown:        make(chan bool, 1),
		shutdownSuccess: make(chan bool, 1),
		Flags:           &Flags{},
	}
	for _, mod := range mods {
		if err := mod(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

type DNSModifier func(s *DnsServer) error

func WithLogger(l logger.Logger) DNSModifier {
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
		s.Domains = domains
		return nil
	}
}

func WithTCPListener(l net.Listener) DNSModifier {
	return func(s *DnsServer) error {
		s.Flags.TCPListener = l
		return nil
	}
}

func WithUDPListener(l net.PacketConn) DNSModifier {
	return func(s *DnsServer) error {
		s.Flags.UDPListener = l
		return nil
	}
}
func WithHost(ip string) DNSModifier {
	return func(s *DnsServer) error {
		s.host = ip
		return nil
	}
}
