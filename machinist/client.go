package machinist

import (
	"context"
	"github.com/enfabrica/enkit/lib/logger"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"time"
)

func NewClient(Username string, client machinist_rpc.UserClient, logger logger.Logger) *Client {
	return &Client{
		Username: Username, MachinistClient: client, Log: logger,
	}
}

type Client struct {
	Username        string
	Log             logger.Logger
	MachinistClient machinist_rpc.UserClient
}

func (c *Client) ReserveMachine(ctx context.Context, name string, start time.Time, end time.Duration) error {
	req := &machinist_rpc.ReserveRequest{
		Type:  machinist_rpc.ReserveRequestType_Reserve,
		Start: start.Unix(),
		End:   start.Add(end).Unix(),
		Name:  []byte(name),
		User:  []byte(c.Username),
	}
	_, err := c.MachinistClient.Reserve(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ListMachines(ctx context.Context, start, end *time.Time) ([]*machinist_rpc.ReservedMachine, error) {
	if start == nil {
		t := time.Date(1969, time.July, 20, 4, 17, 0, 0, time.UTC)
		start = &t
	}
	if end == nil {
		t := time.Now().Add(time.Hour * 200) // to infinity and beyond
		end = &t
	}
	req := &machinist_rpc.ReserveRequest{
		Type:  machinist_rpc.ReserveRequestType_List,
		Start: start.Unix(),
		End:   end.Unix(),
		User:  []byte(c.Username),
	}
	res, err := c.MachinistClient.Reserve(ctx, req)
	if err != nil {
		return nil, err
	}
	return res.Machines, err
}
