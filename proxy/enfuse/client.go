package enfuse

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"crypto/tls"
	fusepb "github.com/enfabrica/enkit/proxy/enfuse/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
	"strconv"
)

var (
	_ fs.FS = &FuseClient{}
)

func NewClient(config *ConnectConfig) (*FuseClient, error) {
	var grpcDialOpts []grpc.DialOption
	if config.ClientCredentials != nil {
		grpcDialOpts = append(grpcDialOpts, grpc.WithTransportCredentials(
			credentials.NewTLS(
				&tls.Config{
					Certificates:             []tls.Certificate{config.Certificate},
					RootCAs:                  config.RootCAs,
					ClientAuth:               tls.RequireAndVerifyClientCert,
					ServerName:               config.ServerName,
					ClientCAs:                config.ClientCredentials,
					InsecureSkipVerify:       false,
					PreferServerCipherSuites: true,
				},
			),
		))
	} else {
		grpcDialOpts = append(grpcDialOpts, grpc.WithInsecure())
	}
	conn, err := grpc.Dial(net.JoinHostPort(config.Url, strconv.Itoa(config.Port)), grpcDialOpts...)
	if err != nil {
		return nil, err
	}
	return &FuseClient{ConnClient: fusepb.NewFuseControllerClient(conn), ConnConfig: config}, nil
}

func MountDirectory(mountPath string, client *FuseClient) error {
	c, err := fuse.Mount(
		mountPath,
	)
	if err != nil {
		return err
	}
	srv := fs.New(c, nil)
	return srv.Serve(client)
}

type FuseClient struct {
	ConnClient fusepb.FuseControllerClient
	ConnConfig *ConnectConfig
}

func (f *FuseClient) Root() (fs.Node, error) {
	return &Dir{Dir: "", Client: f.ConnClient}, nil
}
