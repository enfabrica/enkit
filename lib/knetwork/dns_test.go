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

func TestDNS(t *testing.T) {
	// Setup
	l, err := knetwork.AllocatePort()
	assert.Nil(t, err)

	dnsServer, err := knetwork.NewDNS(
		knetwork.WithDomains([]string{"enkit"}),
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
	time.Sleep(150 * time.Millisecond)

	ips, err := customResolver.LookupHost(context.TODO(), "hello.enkit")
	assert.Nil(t, err)
	assert.Equal(t, 2, len(ips))
}
