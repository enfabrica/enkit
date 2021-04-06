package kdns

import (
	"fmt"
	"github.com/miekg/dns"
	"github.com/pkg/errors"
)

type DnsController struct {
	// aRecords
	aRecords       map[string][]dns.RR
	aRecordChannel chan dns.RR

	txtRecords       map[string][]dns.RR
	txtRecordChannel chan dns.RR
}

type routeHolder struct {
	records chan []dns.RR
	error   chan error
}

// Start will spin up the controller and begin handling dns data requests. It is non blocking
// and returns an error channel which writes if the controller ever ends fatally

func (dc *DnsController) Start() chan error {
	errChan := make(chan error, 1)
	go func() {
		for {
			select {
			case aRecord := <-dc.ARecordChannel:
				fmt.Println("added a record", aRecord)
			}
		}
	}()
	go func() {
		for {
			select {
			case txtRecord := <-dc.TxtRecordChannel:
				fmt.Println("txt record", txtRecord)
				break
			case dc.TxtRecordChannel <- "":

			}
		}
	}()
	return errChan
}

func (dc *DnsController) FetchRecord(t dns.Type, origin string) (chan []dns.RR, chan error) {

}

func (dc *DnsController) AddRecord(rr dns.RR) error {
	switch rr.Header().Rrtype {
	case dns.TypeA:
		dc.aRecordChannel <- rr
	case dns.TypeTXT:
		dc.txtRecordChannel <- rr
	default:
		return fmt.Errorf("kdns currently does not support record type %s", rr.Header().String())
	}
	return nil
}


func (dc *DnsController) AddTxtEntry(d string) {

}

func (dc DnsController) RemoveTxtEntry() {

}
