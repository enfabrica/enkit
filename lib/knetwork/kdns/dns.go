package kdns

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/goroutine"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/miekg/dns"
	"net"
	"strconv"
)

type DnsServer struct {
	Flags   *Flags
	Logger  logger.Logger
	Port    int
	Domains []string

	host       string
	dnsServers []*dns.Server

	readOnlyChan chan struct {
		Return chan *RecordController
		Origin string
	}

	newOrExistingChan chan struct {
		Return chan *RecordController
		Origin string
	}

	shutdown        chan bool
	shutdownSuccess chan bool
}

func (s *DnsServer) DnsFlags() *Flags {
	return s.Flags
}

// Run starts the server and is blocking. It will return an error on close if it did not exit gracefully. To close the
// DnsServer gracefully, call Stop.
func (s *DnsServer) Run() error {
	mux := dns.NewServeMux()
	for _, domain := range s.Domains {
		mux.HandleFunc(dns.Fqdn(domain), s.HandleIncoming)
	}
	portAddr := net.JoinHostPort(s.host, strconv.Itoa(s.Port))
	go s.HandleControllers()
	tcpServer := &dns.Server{Handler: mux, ReusePort: true, Net: "tcp", Addr: portAddr, Listener: s.Flags.TCPListener}
	udpServer := &dns.Server{Handler: mux, ReusePort: true, Net: "udp", Addr: portAddr}
	s.dnsServers = append(s.dnsServers, tcpServer, udpServer)
	return goroutine.WaitFirstError(
		func() error {
			s.Logger.Infof("Serving Dns via udp on %s for domains %v", udpServer.Addr, s.Domains)
			return udpServer.ListenAndServe()
		},
		func() error {
			s.Logger.Infof("Serving Dns via tcp on %s for domains %v", tcpServer.Addr, s.Domains)
			if tcpServer.Listener != nil {
				return tcpServer.ActivateAndServe()
			}
			return tcpServer.ListenAndServe()
		},
	)
}

func (s *DnsServer) Stop() error {
	s.shutdown <- true
	<-s.shutdownSuccess
	var errs []error
	for _, srv := range s.dnsServers {
		if err := srv.Shutdown(); err != nil {
			errs = append(errs, err)
		}
	}
	return multierror.New(errs)
}

// AddEntry will append a entries to domain.
func (s *DnsServer) AddEntry(name string, rr dns.RR) {
	c := s.NewControllerForName(dns.CanonicalName(name))
	c.AddRecords([]dns.RR{rr})
}

// SetEntry will hard replace an entry. Consider it a force AddEntry.
func (s *DnsServer) SetEntry(name string, records []dns.RR) {
	c := s.NewControllerForName(dns.CanonicalName(name))
	c.SetRecords(records)
}

// RemoveFromEntry will delete any entries that container the keywords in the record type.
func (s *DnsServer) RemoveFromEntry(name string, keywords []string, rType uint16) {
	c := s.NewControllerForName(dns.CanonicalName(name))
	c.DeleteRecords(keywords, rType)
}

// ControllerForName will return the controller specified for a specific domain or subdomain. If it does not exist, it
// will return nil.
func (s *DnsServer) ControllerForName(origin string) *RecordController {
	returnChan := make(chan *RecordController, 1)
	s.readOnlyChan <- struct {
		Return chan *RecordController
		Origin string
	}{Return: returnChan, Origin: dns.CanonicalName(origin)}
	return <-returnChan
}

// NewControllerForName will return the controller specified for a specific domain or subdomain. If it does not exist, it
// will create a new one.
func (s *DnsServer) NewControllerForName(origin string) *RecordController {
	returnChan := make(chan *RecordController, 1)
	s.newOrExistingChan <- struct {
		Return chan *RecordController
		Origin string
	}{Return: returnChan, Origin: dns.CanonicalName(origin)}
	return <-returnChan
}

func (s *DnsServer) HandleControllers() {
	controllerMap := map[string]*RecordController{}
	defer close(s.readOnlyChan)
	defer close(s.newOrExistingChan)
	defer func() {
		fmt.Println("closing controllers??", s.Domains)
	}()
	for {
		select {
		case o := <-s.newOrExistingChan:
			if controllerMap[o.Origin] == nil {
				controllerMap[o.Origin] = NewRecordController(s.Logger)
			}
			o.Return <- controllerMap[o.Origin]
		case o := <-s.readOnlyChan:
			o.Return <- controllerMap[o.Origin]
		case <-s.shutdown:
			for _, c := range controllerMap {
				c.Close()
			}
			s.shutdownSuccess <- true
			return
		}
	}
}

// HandleIncoming is the entry point to the dns server, no request logic lives here, only dns.Server specific configs.
func (s *DnsServer) HandleIncoming(writer dns.ResponseWriter, incoming *dns.Msg) {
	m := &dns.Msg{}
	m.SetReply(incoming)
	m.Compress = true
	m.RecursionAvailable = false
	switch incoming.Opcode {
	case dns.OpcodeQuery:
		s.ParseDNS(m)
	}
	err := writer.WriteMsg(m)
	if err != nil {
		s.Logger.Errorf("%s", err)
	}
}

// ParseDNS will only handle dns requests from Domains that it is specified to handle. It will modify the *dns.Msg in place.
func (s *DnsServer) ParseDNS(m *dns.Msg) {
	for _, q := range m.Question {
		c := s.ControllerForName(q.Name)
		if c != nil {
			rrs := c.FetchRecords(q.Qtype)
			for _, txt := range rrs {
				m.Answer = append(m.Answer, txt)
			}
		}
	}
}
