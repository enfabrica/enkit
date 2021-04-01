package machinist_test

import (
	"context"
	"github.com/enfabrica/enkit/machinist"
	"github.com/enfabrica/enkit/machinist/mnode"
	"github.com/enfabrica/enkit/machinist/mserver"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
	"time"
)

func TestJoinServerAndPoll(t *testing.T) {
	lis := bufconn.Listen(2048 * 2048)
	mController, err := mserver.NewController()
	assert.Nil(t, err)
	s, err := mserver.New(
		mserver.WithController(mController),
		mserver.WithMachinistFlags(
			machinist.WithListener(lis),
			machinist.WithInsecure(),
		))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, s.Run())
	}()
	time.Sleep(50 * time.Millisecond)
	defer func() {
		assert.Nil(t, s.Stop())
	}()

	customConnectCtx := context.TODO()
	customConnect := func() (*grpc.ClientConn, error) {
		return grpc.DialContext(customConnectCtx, "bufnet",
			grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
				return lis.Dial()
			}), grpc.WithInsecure())
	}

	n, err := mnode.New(
		mnode.WithDialFunc(customConnect),
		mnode.WithName("test-01"),
		mnode.WithTags([]string{"big", "heavy"}),
		mnode.WithMachinistFlags(
			machinist.WithListener(lis)))

	assert.Nil(t, err)
	go func() {
		assert.Nil(t, n.BeginPolling())
	}()
	time.Sleep(2 * time.Second)
	assert.Equal(t, 1, len(mController.Nodes()))
	assert.NotNil(t, mController.Node("test-01"))

	for _, v := range mController.Nodes() {
		assert.Equal(t, []string{"big", "heavy"}, v.Tags)
		assert.Equal(t, "test-01", v.Name)
	}
}
