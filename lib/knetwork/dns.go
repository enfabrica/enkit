package knetwork

import (
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/logger/klog"
	"github.com/miekg/dns"
	"net"
	"strconv"
	"sync"
)

var (
	DNSEntryNotExistError = errors.New("the following entry did not exist")
	DNSEntryExistError    = errors.New("the following entry did exist")
)

func NewDNS(mods ...DNSModifier) (*DnsServer, error) {
	defaultLogger, err := klog.New("default", klog.FromFlags(*klog.DefaultFlags()))
	if err != nil {
		return nil, err
	}
	s := &DnsServer{
		routeMap: make(map[string][]string),
		Logger:   defaultLogger,
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

type DnsServer struct {
	dnsServer *dns.Server
	sync.RWMutex
	routeMap map[string][]string
	Port     int
	Logger   *klog.Logger
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
		return DNSEntryNotExistError
	}
	s.Lock()
	s.routeMap[dns.Fqdn(name)] = ips
	s.Unlock()
	return nil
}

func (s *DnsServer) ReplaceEntry(name string, ips []string) error {
	s.Lock()
	s.routeMap[name] = ips
	s.Unlock()
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
		s.Logger.Errorw(err.Error())
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
