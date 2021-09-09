package userplane_test

import (
	"context"
	"github.com/enfabrica/enkit/machinist/polling"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/enfabrica/enkit/machinist/state"
	"github.com/enfabrica/enkit/machinist/userplane"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
	"time"
)

var (
	allMachines = []*machinist.StaticMachine{
		{
			Name: "heavy-machine",
			Ips:  []string{net.ParseIP("10.10.0.1").String()},
			Tags: []string{"heavy", "internal"},
		},
	}
)

func TestUserplane(t *testing.T) {
	buffer := 1024 * 1024
	listener := bufconn.Listen(buffer)

	conn, _ := grpc.DialContext(context.TODO(), "", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())

	s, err := userplane.NewServer(
		userplane.WithListener(listener),
	)
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, s.Serve())
	}()
	defer func() {
		assert.Nil(t, s.Stop())
	}()
	time.Sleep(2 * time.Millisecond)
	userClient := machinist.NewUserPlaneClient(conn)
	stateClient := machinist.NewUserplaneStateClient(conn)

	assert.Nil(t, polling.PushState(context.Background(), stateClient, &state.MachineController{
		Machines: allMachines,
	}))

	assert.Nil(t, polling.PushState(context.Background(), stateClient, &state.MachineController{
		Machines: allMachines,
	}))

	resp, err := userClient.List(context.TODO(), &machinist.ListRequest{
		Limit: -1,
	})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(resp.Machines))
	assert.Equal(t,[]string{"heavy", "internal"}, resp.Machines[0].Tags)
}
