package kdns

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"log"
	"testing"
	"time"
)

func TestController(t *testing.T) {
	defer goleak.VerifyNone(t)
	controller := NewRecordController(logger.DefaultLogger{Printer: log.Printf})
	defer controller.Close()
	var rr []dns.RR
	testTxt := []string{
		"my life for aiur", "for the swarm", "foo bar baz",
	}
	testA := []string{
		"10.0.0.1", "10.0.0.2", "10.0.0.3",
	}

	for _, v := range testTxt {
		r, err := dns.NewRR(fmt.Sprintf("%s TXT %s", "example.com", v))
		assert.Nil(t, err)
		rr = append(rr, r)
	}
	for _, v := range testA {
		r, err := dns.NewRR(fmt.Sprintf("%s A %s", "example.com", v))
		assert.Nil(t, err)
		rr = append(rr, r)
	}
	// adding in some ns records for errors
	r0, err := dns.NewRR(fmt.Sprintf("%s NS %s", "example.com", "ns.ns.com"))
	assert.Nil(t, err)
	r1, err := dns.NewRR(fmt.Sprintf("%s NS %s", "example.com", "ns.exmple.com"))
	assert.Nil(t, err)
	rr = append(rr, r0, r1)

	controller.AddRecords(rr)
	aRecords := controller.FetchRecords(dns.TypeA)
	txtRecords := controller.FetchRecords(dns.TypeTXT)
	assert.Equal(t, 3, len(txtRecords))
	fmt.Println(aRecords)
	assert.Equal(t, 3, len(aRecords))

	controller.AddRecords(rr)
	aRecords = controller.FetchRecords(dns.TypeA)
	assert.Equal(t, 6, len(aRecords))
	for _, v := range aRecords {
		assert.Equal(t, dns.TypeA, v.Header().Rrtype)
		_, ok := v.(*dns.A)
		assert.Equal(t, true, ok)
	}
	txtRecords = controller.FetchRecords(dns.TypeTXT)
	for _, v := range txtRecords {
		assert.Equal(t, dns.TypeTXT, v.Header().Rrtype)
		_, ok := v.(*dns.TXT)
		assert.Equal(t, true, ok)
	}
	assert.Equal(t, 6, len(txtRecords))
	// i'm running this because I think there might be a memleak that I couldn't find on pprof. I might be paranoid or
	// the behaviour of unbuffered channels have changed

	// Test Delete single
	controller.DeleteRecords([]string{"10.0.0.3"}, dns.TypeA)
	aRecords = controller.FetchRecords(dns.TypeA)
	assert.Equal(t, 4, len(aRecords))

	// Test delete multiple
	controller.DeleteRecords([]string{"aiur", "swarm"}, dns.TypeTXT)
	txtRecords = controller.FetchRecords(dns.TypeTXT)
	assert.Equal(t, 2, len(txtRecords))

	// Test NoOp
	controller.DeleteRecords([]string{"aiur", "swarm"}, dns.TypeA)
	aRecords = controller.FetchRecords(dns.TypeA)
	assert.Equal(t, 4, len(aRecords))

	// Test Force
	var setIpds []dns.RR
	for _, v := range testA {
		r, _ := dns.NewRR(fmt.Sprintf("%s A %s", "example.com", v))
		setIpds = append(rr, r)
	}
	controller.SetRecords(setIpds)
	aRecords = controller.FetchRecords(dns.TypeA)
	assert.Equal(t, 4, len(aRecords))

	// Test Edit
	r3, _ := dns.NewRR(fmt.Sprintf("%s A %s", "meow.com", "192.168.0.1"))
	r4, _ := dns.NewRR(fmt.Sprintf("%s A %s", "meow.com", "192.168.1.1"))
	editIps := []dns.RR{r3, r4}
	controller.EditRecords(editIps, []string{"10.0.0.1"})
	aRecords = controller.FetchRecords(dns.TypeA)

	assert.Contains(t, aRecords, r3)
	assert.Contains(t, aRecords, r4)

	fmt.Println("records are", aRecords)
	// Delete 1 add 2
	assert.Equal(t, 5, len(aRecords))
	time.Sleep(1 * time.Second)
	controller = nil
}
