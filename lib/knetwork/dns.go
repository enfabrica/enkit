package knetwork

import (
	"github.com/miekg/dns"
	"errors"
	"strconv"
	"sync"
)

var (
	DNSEntryNotExistError = errors.New("the following entry did not exist")
)

func NewDNS() *DnsServer {
	return &DnsServer{
		routeMap:  make(map[string][]string),
		dnsServer: dns.NewServeMux(),
	}
}

func WithLogger() {

}

type DnsServer struct {
	dnsServer *dns.ServeMux
	sync.RWMutex
	routeMap map[string][]string
	Port     int
}

func (s *DnsServer) Start() error {
	s.dnsServer.HandleFunc("service.", s.HandleIncoming)
	return dns.ListenAndServe(":"+strconv.Itoa(s.Port), "udp", s.dnsServer)
}

func (s *DnsServer) Stop() error {

}

func (s *DnsServer) AddEntry() error {

}

func (s *DnsServer) ReplaceEntry() error {

}

func AppendToEntry() error {

}

func (s *DnsServer) RemoveIPFromEntry() error {

}

func RemoveEntry() error {

}

func (s DnsServer) HandleIncoming(writer dns.ResponseWriter, incoming *dns.Msg) {
	m := &dns.Msg{}
	m.SetReply(incoming)
	m.Compress = false

	switch incoming.Opcode {
	case dns.OpcodeQuery:
		parseQuery(m)
	}
	err := writer.WriteMsg(m)
}

func ParseDNS() {

}
