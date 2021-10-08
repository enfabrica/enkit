package enfuse

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"fmt"
	fusepb "github.com/enfabrica/enkit/proxy/enfuse/rpc"
	"google.golang.org/grpc"
)

var (
	_ fs.FS = &FuseClient{}
)

func NewClient(config *ConnectConfig) (*FuseClient, error) {
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", config.Url, config.Port), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &FuseClient{fusepb.NewFuseControllerClient(conn)}, nil
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
}

func (f *FuseClient) Root() (fs.Node, error) {
	return &Dir{Dir: "", Client: f.ConnClient}, nil
}
