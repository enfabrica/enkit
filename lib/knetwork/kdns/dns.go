package kdns

import (
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/miekg/dns"
	"net"
	"strconv"
)

type DnsServer struct {
	Listener net.Listener
	Logger   logger.Logger
	Port     int
	// recordControllers is a read-only map that only gets populated at start. It contains controllers for all the domains
	// that are supported
	domains   []string
	host      string
	dnsServer *dns.Server

	// controllerMap contains all controllers
	requestControllerChan chan struct {
		Return chan *RecordController
		Origin string
	}
}

// Run starts the server and is blocking. It will returning an error on close if it did not exist gracefully. To close the
// DnsServer gracefully, call Stop
func (s *DnsServer) Run() error {
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
	go s.HandleControllers()
	return s.dnsServer.ListenAndServe()
}

func (s *DnsServer) Stop() error {
	return s.dnsServer.Shutdown()
}

func logControllerError(log logger.Logger, errChan chan RecordControllerErr) {
	for {
		e := <-errChan
		log.Errorf("%s", e)
	}
}

// AddEntry will append a
func (s *DnsServer) AddEntry(name string, rr dns.RR) {
	cleanedName := dns.Fqdn(name)
	c := s.ControllerForName(cleanedName)
	c.AddRecords([]dns.RR{rr})
}

// SetEntry will hard replace an entry. Consider it a force AddEntry
func (s *DnsServer) SetEntry(name string, records []dns.RR) {
	cleanedName := dns.Fqdn(name)
	c := s.ControllerForName(cleanedName)
	c.SetRecords(records)
}

// RemoveFromEntry will delete any entries that container the keywords in the record type.
func (s *DnsServer) RemoveFromEntry(name string, keywords []string, rType uint16) {
	cleanedName := dns.Fqdn(name)
	c := s.ControllerForName(cleanedName)
	c.DeleteRecords(keywords, rType)
}

//ControllerForName will return the controller specified for a specific domain or subdomain
func (s *DnsServer) ControllerForName(origin string) *RecordController {
	returnChan := make(chan *RecordController, 1)
	s.requestControllerChan <- struct {
		Return chan *RecordController
		Origin string
	}{Return: returnChan, Origin: dns.Fqdn(origin)}
	return <-returnChan
}

func (s DnsServer) HandleControllers() {
	controllerMap := map[string]*RecordController{}
	for {
		select {
		case o := <-s.requestControllerChan:
			if controllerMap[o.Origin] == nil {
				c := NewRecordController()
				go logControllerError(s.Logger, c.ErrorChan)
				controllerMap[o.Origin] = c
			}
			o.Return <- controllerMap[o.Origin]
		}
	}
}

// HandleIncoming is the entry point to the dns server, no request logic lives here, only dns.Server specific configs.
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

// ParseDNS will only handle dns requests from domains that it is specified to handle. I will modify the *dns.Msg inplace
func (s *DnsServer) ParseDNS(m *dns.Msg) {
	for _, q := range m.Question {
		rrs := s.ControllerForName(q.Name).FetchRecords(q.Qtype)
		for _, txt := range rrs {
			m.Answer = append(m.Answer, txt)
		}
	}
}
