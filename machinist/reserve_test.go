package machinist_test

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/lib/knetwork/kdns"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/machinist"
	"github.com/enfabrica/enkit/machinist/mnode"
	"github.com/enfabrica/enkit/machinist/mserver"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
	"time"
)

func TestReservation(t *testing.T) {
	pp, err := knetwork.AllocatePort()
	assert.Nil(t, err)
	dnsP, _ := pp.Address()
	assert.Nil(t, err)
	c, err := mserver.NewController(
		mserver.DnsPort(dnsP.Port),
		mserver.WithKDnsFlags(
			kdns.WithDomains([]string{"pokemon."}),
			kdns.WithListener(pp),
		))
	assert.Nil(t, err)
	lis := bufconn.Listen(2048 * 2048)
	s, err := mserver.New(
		mserver.WithController(c),
		mserver.WithMachinistFlags(
			machinist.WithListener(lis),
			machinist.WithInsecure(),
		),
	)
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, s.Run())
	}()
	time.Sleep(50 * time.Millisecond)
	//defer func() {
	//	assert.Nil(t, s.Stop())
	//}()
	custCtx := context.TODO()
	customConnect := func() (*grpc.ClientConn, error) {
		return grpc.DialContext(custCtx, "bufnet",
			grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
				return lis.Dial()
			}), grpc.WithInsecure())
	}

	go joinNodeToMaster(t, []mnode.NodeModifier{
		mnode.WithDialFunc(customConnect),
		mnode.WithName("pikachu"),
		mnode.WithIps([]string{"10.0.0.1"}),
		mnode.WithTags([]string{"yellow", "small"}),
		mnode.WithMachinistFlags(
			machinist.WithListener(lis)),
	})
	go joinNodeToMaster(t, []mnode.NodeModifier{
		mnode.WithDialFunc(customConnect),
		mnode.WithName("porygon"),
		mnode.WithIps([]string{"10.0.0.2"}),
		mnode.WithTags([]string{"pink", "white"}),
		mnode.WithMachinistFlags(
			machinist.WithListener(lis)),
	})

	time.Sleep(150 * time.Millisecond)
	fmt.Println(s.Controller.Nodes())
	conn, err := customConnect()
	assert.Nil(t, err)
	mconn := machinist_rpc.NewUserClient(conn)
	ctx, _ := context.WithTimeout(context.TODO(), 2*time.Second)
	mClient := machinist.NewClient("ash", mconn, logger.NilLogger{})

	ms, err := mClient.ListMachines(ctx, nil, nil)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(ms))
	err = mClient.ReserveMachine(context.TODO(), "pikachu", time.Now(), 2 * time.Hour)
	assert.Nil(t, err)
	ms, err = mClient.ListMachines(context.TODO(), nil, nil)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(ms))
	assert.Equal(t, "10.0.0.1", string(ms[0].Ip[0]))
	assert.Equal(t, "pikachu", string(ms[0].Name))
	// Assert you cant re reserve machines
	err = mClient.ReserveMachine(context.TODO(), "pikachu", time.Now(), 2 * time.Hour)
	assert.NotNil(t, err)
}
