package machinist
//
//import (
//	"context"
//	"fmt"
//	"github.com/enfabrica/enkit/lib/client"
//	"github.com/enfabrica/enkit/lib/logger"
//	"github.com/enfabrica/enkit/lib/retry"
//	"github.com/enfabrica/enkit/machinist/rpc/machinist"
//	"os"
//)
//
//type Machinist struct {
//	log logger.Logger
//
//	smods  []client.GwcOrGrpcOptions
//	server *client.ServerFlags
//
//	rmods   retry.Modifiers
//	retrier *retry.Options
//}
//
//func New(mods ...Modifier) (*Machinist, error) {
//	m := &Machinist{log: logger.Nil}
//	if err := Modifiers(mods).Apply(m); err != nil {
//		return nil, err
//	}
//	if m.server == nil {
//		return nil, fmt.Errorf("Server parameters must be supplied")
//	}
//	if m.retrier == nil {
//		mods := append(retry.Modifiers{
//			retry.WithLogger(m.log), retry.WithDescription(fmt.Sprintf("connecting to %s", m.server.Server)),
//		}, m.rmods...)
//
//		m.retrier = retry.New(mods...)
//	}
//	return m, nil
//}
//
//func (m *Machinist) Send(stream machinist.Controller_PollClient, req *machinist.PollRequest) error {
//	// TODO: accumulate requests, check for result.
//
//	if err := stream.Send(req); err != nil {
//		// FIXME dispatch error
//		return nil
//	}
//	return nil
//}
//
//func (m *Machinist) Dispatch(in *machinist.PollResponse) {
//
//}
//
//func (m *Machinist) RegisterRequest() (*machinist.PollRequest, error) {
//
//
//	return req, nil
//}
//
//
//func (m *Machinist) Run() error {
//	return m.retrier.Run(func() error {
//		//conn, err := m.server.Connect(m.smods...)
//		//if err != nil {
//		//	return err
//		//}
//		//// FIXME
//		////defer conn.Close()
//		//
//		//client := machinist.NewControllerClient(conn)
//		//stream, err := client.Poll(context.Background())
//		//if err != nil {
//		//	return err
//		//}
//		//
//		//req, err := m.RegisterRequest()
//		//if err != nil {
//		//	return err
//		//}
//		//
//		//m.Send(stream, req)
//		//
//		//for {
//		//	in, err := stream.Recv()
//		//	if err != nil {
//		//		// Check for io.EOF
//		//		return err
//		//	}
//		//	m.Dispatch(in)
//		//}
//		return nil
//	})
//}
