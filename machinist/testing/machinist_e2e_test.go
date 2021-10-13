package e2e_test

import (
	"context"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/machinist/config"
	"github.com/enfabrica/enkit/machinist/machine"
	"github.com/enfabrica/enkit/machinist/mserver"
	"github.com/enfabrica/enkit/machinist/state"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TODO (adam): make the machinist factory less ugly
// TODO (adam): write e2e testing docs
// TODO (adam): run e2e testing over the wire, buffcon for unit tests
func TestJoinServerAndPoll(t *testing.T) {
	dnsLis, customResolver := registerPort(t)
	dnsAddr, err := dnsLis.Address()
	assert.Nil(t, err)

	lis := bufconn.Listen(2048 * 2048)

	rng := rand.New(srand.Source)
	stateFileName := filepath.Join(os.TempDir(), strconv.Itoa(rng.Int())+".json")
	s, mController, err := createNewControlPlane(t, []mserver.ControllerModifier{
		mserver.WithStateWriteDuration("50ms"),
		mserver.WithKDnsFlags(
			kdns.WithTCPListener(dnsLis),
			kdns.WithHost(dnsAddr.IP.String()),
			kdns.WithPort(dnsAddr.Port),
			kdns.WithDomains([]string{"enkit.", "enkitdev."}),
		),
		mserver.WithStateFile(stateFileName),
	}, []mserver.Modifier{
		mserver.WithMachinistFlags(
			config.WithListener(lis),
			config.WithInsecure(),
		),
	})
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
	// IMPORTANT TIME TTL, LETS CONTROLPLANE WRITE AND DO ASYNC ACTIVITIES
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, 2, len(mController.Nodes()))
	assert.NotNil(t, state.GetMachine(mController.State, "test02"))
	assert.NotNil(t, state.GetMachine(mController.State, "test01"))

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
	assert.Nil(t, lis.Close())
	time.Sleep(20 * time.Millisecond)

	//Test serialization
	dnsLis, customResolver = registerPort(t)
	dnsAddr, err = dnsLis.Address()
	assert.NoError(t, err)
	lis = bufconn.Listen(2048 * 2048)
	mainServer, _, err := createNewControlPlane(t, []mserver.ControllerModifier{
		mserver.WithStateWriteDuration("50ms"),
		mserver.WithKDnsFlags(
			kdns.WithTCPListener(dnsLis),
			kdns.WithHost(dnsAddr.IP.String()),
			kdns.WithPort(dnsAddr.Port),
			kdns.WithDomains([]string{"enkit.", "enkitdev."}),
		),
		mserver.WithStateFile(stateFileName),
	}, []mserver.Modifier{
		mserver.WithMachinistFlags(
			config.WithListener(lis),
			config.WithInsecure(),
		),
	})
	assert.Nil(t, err)
	go func() {
		assert.NoError(t, mainServer.Run())
	}()
	time.Sleep(50 * time.Millisecond)
	res, err = customResolver.LookupHost(context.TODO(), "test01.enkitdev")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, "10.0.0.4", res[0])
	tagsRes, err = customResolver.LookupTXT(context.TODO(), "test01.enkit")
	assert.Nil(t, err)
	assert.Equal(t, []string{"big", "heavy"}, tagsRes)
	//assert.Nil(t, mainServer.Stop())
}

func joinNodeToMaster(t *testing.T, opts []machine.NodeModifier) *machine.Machine {
	n, err := machine.New(opts...)
	assert.NoError(t, err)
	assert.Nil(t, n.Init())
	go func() {
		assert.Nil(t, n.BeginPolling())
	}()
	return n
}

func createNewControlPlane(t *testing.T, mods []mserver.ControllerModifier, cmods []mserver.Modifier) (*mserver.ControlPlane, *mserver.Controller, error) {
	mController, err := mserver.NewController(mods...)
	assert.Nil(t, err)
	s, err := mserver.New(append(cmods, mserver.WithController(mController))...)
	return s, mController, err
}

func registerPort(t *testing.T) (*knetwork.PortDescriptor, *net.Resolver) {
	machinistDnsPort, err := knetwork.AllocatePort()
	assert.Nil(t, err)
	customResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, "tcp", machinistDnsPort.Addr().String())
		},
	}
	return machinistDnsPort, customResolver
}
