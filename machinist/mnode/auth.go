package mnode

import (
	"context"
	"encoding/pem"
	"fmt"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/enauth"
	"github.com/enfabrica/enkit/lib/kcerts"
	"golang.org/x/crypto/ssh"
)

func (n *Node) Enroll(user string) error {
	_, err := enauth.PerformLogin(n.AuthClient, n.Log, n.Repeater, user)
	if err != nil {
		return err
	}
	priv, pub, err := kcerts.MakeKeys()
	if err != nil {
		return err
	}
	hreq := &auth.HostCertificateRequest{
		Hosts:    n.nf.DnsNames,
		Hostcert: pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: ssh.MarshalAuthorizedKey(pub)}),
	}
	resp, err := n.AuthClient.HostCertificate(context.TODO(), hreq)
	if err != nil {
		return err
	}
	fmt.Println("huzzah success")
	fmt.Println(resp.Capublickey)
	fmt.Println(resp.Signedhostcert)
	fmt.Println(priv)
	return nil
}
