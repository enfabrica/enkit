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

type FuseDir struct {
	Raw enfuse.FileInfoResponse
}

func (f *FuseDir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	fmt.Println("looking up in fusedir name")
	if req.Name == "hello" {
		return FuseFiles{}, nil
	}
	return nil, syscall.ENOENT
}

func (f *FuseDir) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Inode = 2
	attr.Mode = os.ModeDir | 0o555
	return nil
}

func (f *FuseDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	return []fuse.Dirent{{
		Inode: 5,
		Type:  fuse.DT_File,
		Name:  "hello",
	}}, nil
}

type FuseFiles struct{}

func (f FuseFiles) ReadAll(ctx context.Context) ([]byte, error) {
	return []byte(greeting), nil
}

const greeting = "hello, world\n"

func (f FuseFiles) Attr(ctx context.Context, attr *fuse.Attr) error {
	fmt.Println("atter is ", attr)
	attr.Inode = 2
	attr.Mode = 0o444
	attr.Size = uint64(len(greeting))
	return nil
}