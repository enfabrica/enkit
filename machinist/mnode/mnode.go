package mnode

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/enfabrica/enkit/lib/client/ccontext"
	machinistRpc "github.com/enfabrica/enkit/machinist"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"time"
)

type NodeErr struct {
	err error
}
type Node struct {
	Client      machinist.ControllerClient
	TLSCert     string
	Connection  *grpc.ClientConn
	KillChannel chan NodeErr
	StopChannel chan bool
	PongChannel chan *machinist.PollResponse
	*prometheus.Registry
	*ccontext.Context
}

func New(nm ...NodeModifier) (*Node, error) {
	n := &Node{
		KillChannel: make(chan NodeErr),
		Context:     ccontext.DefaultContext(),
		Registry:    prometheus.NewRegistry(),
	}
	for _, mod := range nm {
		if err := mod(n); err != nil {
			fmt.Println("there was an error", err.Error())
			return nil, err
		}
	}
	return n, nil
}

type NodeModifier func(node *Node) error


func (n *Node) listenForPong() {
	ctx := context.Background()
	pollStream, err := n.Client.Poll(ctx)
	if err != nil {
		n.Kill(err)
	}
	for {
		pollResponse, err := pollStream.Recv()
		if err != nil {
			n.Kill(err)
		}
		n.PongChannel <- pollResponse
	}

}

func (n *Node) BeginPolling() {
	collector := prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{})
	err := n.Registry.Register(collector)

	ctx := context.Background()
	pollStream, err := n.Client.Poll(ctx)
	if err != nil {
		panic(err)
	}
	err = pollStream.Send(&machinist.PollRequest{
		Req: &machinist.PollRequest_Ping{
			Ping: &machinist.ClientPing{
				Payload: []byte("data here"),
			},
		},
	})
	if err != nil {
		n.Kill(err)
	}
	fmt.Println("in here")
	for {
		select {
		case nodeErr := <-n.KillChannel:
			fmt.Printf("killing node with error %v", nodeErr)
			return
		//TODO(adam): include symmetic encodeer token as extra layer of security in case node is comped
		case pollResp := <-n.PongChannel:
			fmt.Println("recv %v", pollResp)
			// TODO(adam): should scheduling happen here or should it happen somewhere else
		case <-time.After(2 * time.Second):
			err = pollStream.Send(&machinist.PollRequest{
				Req: &machinist.PollRequest_Ping{
					Ping: &machinist.ClientPing{
						Payload: []byte("hello"),
					},
				},
			})
			fmt.Println("sent")
			if err != nil {
				n.Kill(err)
			}
		}
	}
}

func (n *Node) Kill(err error) {
	fmt.Println("killing with err", err.Error())
	n.KillChannel <- NodeErr{}
}

func (n Node) Stop() {
	n.KillChannel <- NodeErr{}
}

func WithInviteToken(token string) NodeModifier {
	return func(node *Node) error {
		data, err := base64.RawStdEncoding.DecodeString(token)
		if err != nil {
			return err
		}
		i := machinistRpc.InvitationToken{}
		err = json.Unmarshal(data, &i)
		if err != nil {
			return err
		}
		p := x509.NewCertPool()
		p.AppendCertsFromPEM([]byte(i.RootCA))
		m, err := tls.X509KeyPair([]byte(i.CRT), []byte(i.PrivateKey))
		if err != nil {
			return err
		}
		//TODO(adam): finish mTLS so we dont have to insecure
		tlsConfig := &tls.Config{
			RootCAs:            p,
			Certificates:       []tls.Certificate{m},
			ServerName:         "machinist",
			InsecureSkipVerify: true,
		}
		transportCredentials := credentials.NewTLS(tlsConfig)
		// TODO(adam): circulate through the ip address in the invite token until one hits
		// right now this only works on localhost
		conn, err := grpc.Dial(fmt.Sprintf(":%d", i.Port), grpc.WithTransportCredentials(transportCredentials))
		if err != nil {
			return err
		}
		node.Connection = conn
		node.Client = machinist.NewControllerClient(conn)
		return nil
	}
}

func (n *Node) ListenAndServe() error {
	go n.BeginPolling()
	go n.listenForPong()
	go n.startPrometheus()
	for {
		select {
		case err := <-n.KillChannel:
			return err.err
		}
	}
}

func (n *Node) startPrometheus() {

}

func (n Node) AttemptToConnect() (grpc.ClientConn, error)  {
	panic("erer")
}