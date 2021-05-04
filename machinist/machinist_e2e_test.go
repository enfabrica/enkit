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

	go joinNodeToMaster(t, []mnode.NodeModifier{
		mnode.WithDialFunc(customConnect),
		mnode.WithName("test-01"),
		mnode.WithTags([]string{"big", "heavy"}),
		mnode.WithMachinistFlags(
			machinist.WithListener(lis)),
	})

	go joinNodeToMaster(t, []mnode.NodeModifier{
		mnode.WithDialFunc(customConnect),
		mnode.WithName("test-02"),
		mnode.WithTags([]string{"teeny", "weeny"}),
		mnode.WithMachinistFlags(
			machinist.WithListener(lis)),
	})

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 2, len(mController.Nodes()))
	assert.NotNil(t, mController.Node("test-02"))
	assert.NotNil(t, mController.Node("test-01"))

	//TODO(adam): table test this
	for _, v := range mController.Nodes() {
		if v.Name == "test-01" {
			assert.Equal(t, []string{"big", "heavy"}, v.Tags)
		} else if v.Name == "test-02" {
			assert.Equal(t, []string{"teeny", "weeny"}, v.Tags)
		} else {
			t.Fatalf("controller found node %v, which should not be present", v)
		}
	}
}

func joinNodeToMaster(t *testing.T, opts []mnode.NodeModifier) *mnode.Node {
	n, err := mnode.New(&mnode.Config{}, opts...)
	assert.Nil(t, err)
	assert.Nil(t, n.Init())
	go func() {
		assert.Nil(t, n.BeginPolling())
	}()
	return n
}
