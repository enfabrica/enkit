package knetwork_test

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
	"time"
)

// TODO(adam): just table test this
func TestDNS(t *testing.T) {
	// Setup
	l, err := knetwork.AllocatePort()
	assert.Nil(t, err)

	dnsServer, err := knetwork.NewDNS(
		knetwork.WithDomains([]string{"enkit.", "enb."}),
		knetwork.WithListener(l),
	)
	fmt.Println(l.Addr().String())
	customResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, l.Addr().String())
		},
	}
	go func() {
		assert.Nil(t, dnsServer.Start())
	}()
	defer dnsServer.Stop()

	// Actual Test
	err = dnsServer.AddEntry("hello.enkit", []string{"10.9.9.9", "10.90.80.70"})
	assert.Nil(t, err)
	err = dnsServer.AddEntry("hello.enb", []string{"10.9.9.9", "10.90.80.70"})
	assert.Nil(t, err)
	time.Sleep(150 * time.Millisecond)

	// Double check that the dns server only uses our domains
	_, err = customResolver.LookupHost(context.TODO(), "something.com")
	assert.NotNil(t, err)

	ips, err := customResolver.LookupHost(context.TODO(), "hello.enkit")
	assert.Nil(t, err)
	assert.Equal(t, 2, len(ips))

	newIps := []string{"10.9.9.9", "10.90.80.70", "10.0.0.1"}
	assert.Nil(t, dnsServer.AppendToEntry("hello.enkit", newIps))
	assert.Nil(t, dnsServer.AppendToEntry("hello.enkit.", newIps))
	assert.Nil(t, dnsServer.AppendToEntry("hello.enb", newIps))
	assert.Nil(t, dnsServer.AppendToEntry("hello.enb.", newIps))

	ips, err = customResolver.LookupHost(context.TODO(), "hello.enkit")
	assert.Nil(t, err)
	assert.Equal(t,newIps, ips)
	assert.Equal(t, 3, len(ips))
	ips, err = customResolver.LookupHost(context.TODO(), "hello.enb")
	assert.Nil(t, err)
	assert.Equal(t,newIps, ips)
	assert.Equal(t, 3, len(ips))

}
