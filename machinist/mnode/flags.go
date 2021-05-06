package mnode

import (
	"github.com/enfabrica/enkit/machinist"
	"google.golang.org/grpc"
)

type NodeModifier func(node *Node) error

func WithInviteToken(token string) NodeModifier {
	return func(node *Node) error {
		return nil
	}
}

func WithMasterServer(url string) NodeModifier {
	return func(node *Node) error {
		return nil
	}
}

func WithAuthServer(url string) NodeModifier {
	return func(node *Node) error {
		return nil
	}
}

func WithMachinistFlags(mods ...machinist.Modifier) NodeModifier {
	return func(n *Node) error {
		for _, mod := range mods {
			if err := mod(n); err != nil {
				return err
			}
		}
		return nil
	}
}

func WithName(name string) NodeModifier {
	return func(node *Node) error {
		node.Name = name
		return nil
	}
}

func WithTags(tags []string) NodeModifier {
	return func(node *Node) error {
		node.Tags = tags
		return nil
	}
}

func WithDialFunc(f func() (*grpc.ClientConn, error)) NodeModifier {
	return func(node *Node) error {
		node.DialFunc = f
		return nil
	}
}

func WithIpsAddresses(ips []string) NodeModifier {
	return func(node *Node) error {
		node.Ips = ips
		return nil
	}
}
