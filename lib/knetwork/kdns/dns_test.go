package kdns_test

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"net"
	"testing"
	"time"
)

// TODO(adam): just table test this
func TestDNS(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Setup
	l, err := knetwork.AllocatePort()
	assert.Nil(t, err)

	dnsServer, err := kdns.NewDNS(
		kdns.WithDomains([]string{"enkit.", "enb."}),
		kdns.WithListener(l),
	)
	defer func() {
		assert.Nil(t, dnsServer.Stop())
	}()
	go func() {
		assert.Nil(t, dnsServer.Run())
	}()
	customResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, l.Addr().String())
		},
	}


	tips := []string{"10.9.9.9", "10.90.80.70"}
	var rrs []dns.RR
	for _, i := range tips {
		r, err := dns.NewRR(fmt.Sprintf("%s A %s", "hello.enkit", i))
		assert.Nil(t, err)
		rrs = append(rrs, r)
	}
	// Actual Test
	go func() {
		dnsServer.AddEntry("hello.enkit", rrs[0])
	}()
	go func() {
		dnsServer.AddEntry("hello.enkit", rrs[1])
	}()
	time.Sleep(150 * time.Millisecond)

	// Double check that the dns server only uses our domains
	_, err = customResolver.LookupHost(context.TODO(), "something.com")
	assert.NotNil(t, err)

	ips, err := customResolver.LookupHost(context.TODO(), "hello.enkit")
	assert.Nil(t, err)
	for _, aa := range ips {
		fmt.Println(aa)
	}
	assert.Equal(t, 2, len(ips))
	//
	newIps := []string{"10.9.9.9", "10.90.80.70", "10.0.0.1"}
	var setIPs []dns.RR
	for _, v := range newIps {
		r, err := dns.NewRR(fmt.Sprintf("%s A %s", "hello.enkit", v))
		assert.Nil(t, err)
		setIPs = append(setIPs, r)
	}
	dnsServer.SetEntry("hello.enkit", setIPs)
	ips, err = customResolver.LookupHost(context.TODO(), "hello.enkit")
	assert.Nil(t, err)
	assert.Equal(t, newIps, ips)
	assert.Equal(t, 3, len(ips))

	dnsServer.RemoveFromEntry("hello.enkit", []string{"10.9.9.9"}, dns.TypeA)
	ips, err = customResolver.LookupHost(context.TODO(), "hello.enkit")
	assert.Nil(t, err)
	assert.Equal(t, 2, len(ips))
	assert.NotContains(t, ips, "10.9.9.9")

}
