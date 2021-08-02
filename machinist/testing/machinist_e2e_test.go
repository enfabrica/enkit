package e2e_test

import (
	"context"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/machinist/config"
	"github.com/enfabrica/enkit/machinist/machine"
	"github.com/enfabrica/enkit/machinist/mserver"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
	"time"
)

func TestJoinServerAndPoll(t *testing.T) {
	machinistDnsPort, err := knetwork.AllocatePort()
	assert.Nil(t, err)
	a, err := machinistDnsPort.Address()
	assert.Nil(t, err)
	customResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, machinistDnsPort.Addr().String())
		},
	}
	lis := bufconn.Listen(2048 * 2048)
	s, mController, err := createNewControlPlane(t, lis, machinistDnsPort, a.Port)
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, s.Run())
	}()

	customConnectCtx := context.TODO()
	customConnect := func() (*grpc.ClientConn, error) {
		return grpc.DialContext(customConnectCtx, "bufnet",
			grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
				return lis.Dial()
			}), grpc.WithInsecure())
	}

	go joinNodeToMaster(t, []machine.NodeModifier{
		machine.WithDialFunc(customConnect),
		machine.WithName("test01"),
		machine.WithIps([]string{"10.0.0.4"}),
		machine.WithTags([]string{"big", "heavy"}),
		machine.WithMachinistFlags(
			config.WithListener(lis),
			config.WithEnableMetrics(false),
		),
	})

	go joinNodeToMaster(t, []machine.NodeModifier{
		machine.WithDialFunc(customConnect),
		machine.WithName("test02"),
		machine.WithIps([]string{"10.0.0.1"}),
		machine.WithTags([]string{"teeny", "weeny"}),
		machine.WithMachinistFlags(
			config.WithListener(lis),
			config.WithEnableMetrics(false),
		),
	})

	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, 2, len(mController.Nodes()))
	assert.NotNil(t, mController.Node("test02"))
	assert.NotNil(t, mController.Node("test01"))

	//TODO(adam): table test this
	for _, v := range mController.Nodes() {
		if v.Name == "test01" {
			assert.Equal(t, []string{"big", "heavy"}, v.Tags)
		} else if v.Name == "test02" {
			assert.Equal(t, []string{"teeny", "weeny"}, v.Tags)
		} else {
			t.Fatalf("controller found node %v, which should not be present", v)
		}
	}
	res, err := customResolver.LookupHost(context.TODO(), "test01.enkitdev")
	assert.Nil(t, err)
	assert.Equal(t, "10.0.0.4", res[0])
	tagsRes, err := customResolver.LookupTXT(context.TODO(), "test01.enkit")
	assert.Nil(t, err)
	assert.Equal(t, []string{"big", "heavy"}, tagsRes)

	assert.Nil(t, s.Stop())
	time.Sleep(2 * time.Millisecond)
}

func joinNodeToMaster(t *testing.T, opts []machine.NodeModifier) *machine.Machine {
	n, err := machine.New(opts...)
	assert.Nil(t, err)
	assert.Nil(t, n.Init())
	go func() {
		assert.Nil(t, n.BeginPolling())
	}()
	return n
}

func createNewControlPlane(t *testing.T, l net.Listener, dnsListener net.Listener, p int) (*mserver.ControlPlane, *mserver.Controller, error) {
	mController, err := mserver.NewController(
		mserver.DnsPort(p),
		mserver.WithKDnsFlags(
			kdns.WithListener(dnsListener),
			kdns.WithDomains([]string{"enkit.", "enkitdev."}),
		),
	)
	assert.Nil(t, err)
	s, err := mserver.New(
		mserver.WithController(mController),
		mserver.WithMachinistFlags(
			config.WithListener(l),
			config.WithInsecure(),
		))
	return s, mController, err
}
