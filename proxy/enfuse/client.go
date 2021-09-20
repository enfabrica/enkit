package enfuse

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"context"
	"fmt"
	enfuse "github.com/enfabrica/enkit/proxy/enfuse/rpc"
	"os"
	"syscall"
)

var (
	_ fs.FS                 = &FuseClient{}
	_ fs.NodeStringLookuper = &FuseClient{}
	_ fs.Node               = &FuseClient{}
)

func MountDirectory(mountPath string, client *FuseClient) error {
	c, err := fuse.Mount(
		mountPath,
		fuse.AllowOther(),
	)
	if err != nil {
		return err
	}
	srv := fs.New(c, nil)
	return srv.Serve(client)
}

type FuseClient struct {
	ConnClient enfuse.FuseControllerClient
}

func (f *FuseClient) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	r, err := f.ConnClient.FileInfo(ctx, &enfuse.FileInfoRequest{Dir: ""})
	if err != nil {
		return nil, err
	}
	return ConvertToDirent(r.Files), nil
}

func (f *FuseClient) Lookup(ctx context.Context, name string) (fs.Node, error) {
	fmt.Println("looking up", name)
	if name == "hello" {
		return FuseFiles{}, nil
	}
	if name == "next" {
		return &FuseDir{}, nil
	}
	return nil, syscall.ENOENT
}

func (f *FuseClient) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Inode = 1
	attr.Mode = os.ModeDir | 0o555
	return nil
}

func (f *FuseClient) Root() (fs.Node, error) {
	return f, nil
}

func ConvertToDirent(info []*enfuse.FileInfo) []fuse.Dirent {
	var fdir []fuse.Dirent
	for _, i := range info {
		dt := fuse.DT_File
		inode := uint64(3)
		if i.IsDir {
			inode = 4
			dt = fuse.DT_Dir
		}
		fdir = append(fdir, fuse.Dirent{
			Inode: inode,
			Type:  dt,
			Name:  i.Name,
		})
	}
	return fdir
}
