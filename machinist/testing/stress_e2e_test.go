package e2e_test

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/machinist/config"
	"github.com/enfabrica/enkit/machinist/machine"
	"github.com/enfabrica/enkit/machinist/mserver"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const NumMachines = 1000

// TestStressDns tests 1000 machines, while under heavy read load and over real ports. It's purpose is to test for deadlocks
// and leaked routines.
func TestStressDns(t *testing.T) {
	machinistDnsPort, _ := registerPort(t)
	grpcLis, _ := registerPort(t)
	grpcAddr, err := grpcLis.Address()
	assert.NoError(t, err)
	dnsAddr, err := machinistDnsPort.Address()
	assert.Nil(t, err)
	rng := rand.New(srand.Source)
	stateFileName := filepath.Join(os.TempDir(), strconv.Itoa(rng.Int())+".json")
	smthElse, _, err := createNewControlPlane(t, []mserver.ControllerModifier{
		mserver.WithStateWriteDuration("50ms"),
		mserver.WithAllRecordsRefreshRate("50ms"),
		mserver.WithKDnsFlags(
			kdns.WithTCPListener(machinistDnsPort),
			kdns.WithHost(dnsAddr.IP.String()),
			kdns.WithPort(dnsAddr.Port),
			kdns.WithDomains([]string{"stress.", "stressdev."}),
		),
		mserver.WithStateFile(stateFileName),
	}, []mserver.Modifier{
		mserver.WithMachinistFlags(
			config.WithListener(grpcLis),
			config.WithInsecure(),
		),
	})
	assert.Nil(t, err)
	go func() {
		err := smthElse.Run()
		assert.Nil(t, err)
	}()
	time.Sleep(50 * time.Millisecond)
	ips := createIps(NumMachines)
	for i := 0; i < NumMachines; i++ {
		joinNodeToMaster(t, []machine.NodeModifier{
			machine.WithName(fmt.Sprintf("stress%d", i)),
			machine.WithIps([]string{ips[i].String()}),
			machine.WithTags([]string{fmt.Sprintf("number%d", i)}),
			machine.WithMachinistFlags(
				config.WithEnableMetrics(false),
				config.WithControlPlaneHost("127.0.0.1"),
				config.WithControlPlanePort(grpcAddr.Port),
			),
		})
	}
	time.Sleep(5 * time.Second)
	assert.NoError(t, err)
	c := dns.Client{

	}
	c.Net = "tcp"
	m := &dns.Msg{}
	m.SetQuestion(dns.CanonicalName("_all.stress"), dns.TypeA)
	portAddr := net.JoinHostPort(dnsAddr.IP.String(), strconv.Itoa(dnsAddr.Port))
	r, _, err := c.Exchange(m, portAddr)
	assert.NoError(t, err)
	assert.Equal(t, NumMachines, len(r.Answer))
	time.Sleep(50 * time.Millisecond)
}

// just makes a bunch of ips that arent the same from each other
func createIps(max int) []net.IP {
	toRet := make([]net.IP, max)
	index := 0
	for first := 0; first < 256; first++ {
		for second := 0; second < 256; second++ {
			for third := 0; third < 256; third++ {
				for fourth := 0; fourth < 256; fourth++ {
					toRet[index] = net.IPv4(byte(first), byte(second), byte(third), byte(fourth))
					index += 1
					if index == max {
						goto DONE
					}
				}
			}
		}
	}
DONE:
	return toRet
}
