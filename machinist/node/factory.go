package mnode

import (
	"fmt"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/enfabrica/enkit/machinist"
	"github.com/enfabrica/enkit/machinist/config"
	"google.golang.org/grpc"
	"log"
	"net"
	"time"
)

func New(mods ...NodeModifier) (*Node, error) {
	n := &Node{
		Log:      logger.DefaultLogger{Printer: log.Printf},
		Repeater: retry.New(retry.WithWait(5*time.Second), retry.WithAttempts(5)),
		Node: &config.Node{

		},
	}
	for _, m := range mods {
		if err := m(n); err != nil {
			return nil, err
		}
	}
	if n.AuthClient == nil && n.DialFunc == nil {
		conn, err := n.config.bf.Connect()
		if err != nil {
			return nil, err
		}
		n.AuthClient = auth.NewAuthClient(conn)
	}
	return n, nil
}

type NodeModifier func(node *Node) error

func WithMachinistFlags(mods ...machinist.Modifier) NodeModifier {
	return func(n *Node) error {
		for _, mod := range mods {
			if err := mod(n.config); err != nil {
				return err
			}
		}
		return nil
	}
}

func WithName(name string) NodeModifier {
	return func(node *Node) error {
		node.config.Name = name
		return nil
	}
}

func WithTags(tags []string) NodeModifier {
	return func(node *Node) error {
		node.config.Tags = tags
		return nil
	}
}

func WithDialFunc(f func() (*grpc.ClientConn, error)) NodeModifier {
	return func(node *Node) error {
		node.DialFunc = f
		return nil
	}
}

func WithAuthFlags(af *client.AuthFlags) NodeModifier {
	return func(node *Node) error {
		cc, err := af.Connect()
		if err != nil {
			return err
		}
		node.AuthClient = auth.NewAuthClient(cc)
		return nil
	}
}

func WithIps(ips []string) NodeModifier {
	return func(node *Node) error {
		var errors []error
		var ipps []net.IP
		for _, v := range ips {
			if i := net.ParseIP(v); i != nil {
				ipps = append(ipps, i)
				continue
			}
			errors = append(errors, fmt.Errorf("%s is not a valid ip", v))
		}
		if len(errors) != 0 {
			return multierror.New(errors)
		}
		node.IpAddresses = ips
		return nil
	}
}
