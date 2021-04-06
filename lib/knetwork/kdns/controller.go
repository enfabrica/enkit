package kdns

import (
	"github.com/miekg/dns"
)

type DnsController struct {
	// routeMap
	routeMap map[string]*BaseRecord
	//
	domains map[string]bool
	host    string
}

type routeHolder struct {
	Origin string
}

// Start will spin up the controller and begin handling dns data requests. It is non blocking
// and returns an error channel which writes if the controller ever ends fatally

func (dc *DnsController) Start() chan error {
	errChan := make(chan error, 1)
	go func() {
		for {
			select {

			}
		}
	}()
	return errChan
}

func (dc *DnsController) FetchRecord(t dns.Type, origin string) ([]dns.RR, error) {

}

func (dc *DnsController) AddAEntry(a string) {

}

func (dc *DnsController) AddTxtEntry(d string) {

}

func (dc DnsController) RemoveTxtEntry() {

}
