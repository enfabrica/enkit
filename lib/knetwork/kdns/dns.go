package kdns

import (
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/miekg/dns"
	"net"
	"strconv"
	"sync"
)

var (
	DNSEntryNotExistError = errors.New("the following entry did not exist")
	DNSEntryExistError    = errors.New("the following entry did exist")
)

type DnsServer struct {
	dnsServer *dns.Server
	sync.RWMutex
	routeMap map[string][]string
	Port     int
	Logger   logger.Logger
	domains  []string
	Listener net.Listener
	host     string
}

func (s *DnsServer) Start() error {
	mux := dns.NewServeMux()
	for _, domain := range s.domains {
		mux.HandleFunc(dns.Fqdn(domain), s.HandleIncoming)
	}
	s.dnsServer = &dns.Server{Handler: mux}
	if s.Listener != nil {
		s.dnsServer.Listener = s.Listener
		s.dnsServer.Net = "udp"
		s.dnsServer.Addr = s.Listener.Addr().String()
	}
	if s.Port != 0 {
		s.dnsServer.Addr = net.JoinHostPort(s.host, strconv.Itoa(s.Port))
	}
	return s.dnsServer.ListenAndServe()
}

func (s *DnsServer) Stop() error {
	return s.dnsServer.Shutdown()
}

func (s *DnsServer) AddEntry(name string, ips []string) error {
	if s.routeMap[dns.Fqdn(name)] != nil {
		return fmt.Errorf("%w: %s", DNSEntryExistError, name)
	}
	s.Lock()
	s.routeMap[dns.Fqdn(name)] = ips
	s.Unlock()
	return nil
}

// ReplaceEntry will hard replace an entry. Consider it a force AddEntry
func (s *DnsServer) ReplaceEntry(name string, ips []string) error {
	s.Lock()
	s.routeMap[name] = ips
	s.Unlock()
	return nil
}

// AppendToEntry will add ips to an existing entry. Any collisions are automatically handled. It will not automatically
// add the entry if it does not exist
func (s *DnsServer) AppendToEntry(name string, ips []string) error {
	if s.routeMap[dns.Fqdn(name)] == nil {
		return fmt.Errorf("%w: %s", DNSEntryNotExistError, name)
	}
	fqdnName := dns.Fqdn(name)
	s.Lock()
	s.routeMap[fqdnName] = appendIfNotPresent(s.routeMap[fqdnName], ips)
	s.Unlock()
	return nil
}

// RemoveIPFromEntry will remove ips from an entry if the entry and name exists. I will not error if the ip to delete
// is not found in th records
func (s *DnsServer) RemoveIPFromEntry(name string, ips []string) error {
	if s.routeMap[name] == nil {
		return fmt.Errorf("%w: %s", DNSEntryNotExistError, name)
	}
	return nil
}

func (s *DnsServer) RemoveEntry(name string) error {
	return nil
}

func (s *DnsServer) HandleIncoming(writer dns.ResponseWriter, incoming *dns.Msg) {
	m := &dns.Msg{}
	m.SetReply(incoming)
	m.Compress = false
	switch incoming.Opcode {
	case dns.OpcodeQuery:
		s.ParseDNS(m)
	}
	err := writer.WriteMsg(m)
	if err != nil {
		s.Logger.Errorf("%s", err)
	}
}

func (s *DnsServer) ParseDNS(m *dns.Msg) {
	for _, q := range m.Question {
		switch q.Qtype {
		case dns.TypeA:
			ips := s.routeMap[q.Name]
			for _, ip := range ips {
				rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip))
				if err == nil {
					m.Answer = append(m.Answer, rr)
				}
			}
		}
	}
}

// generic func
func appendIfNotPresent(s1, s2 []string) (inter []string) {
	hash := make(map[string]bool)
	for _, e := range s1 {
		hash[e] = true
		inter = append(inter, e)
	}
	for _, e := range s2 {
		// If elements present in the hashmap then append intersection list.
		if !hash[e] {
			inter = append(inter, e)
		}
	}
	return inter
}
