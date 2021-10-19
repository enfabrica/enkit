package enfuse

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"context"
	"fmt"
	fusepb "github.com/enfabrica/enkit/proxy/enfuse/rpc"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	_ fs.NodeRequestLookuper = &Dir{}
	_ fs.Node                = &Dir{}

	_ fs.Node         = &File{}
	_ fs.HandleReader = &File{}
)

func ConvertToDirent(info []*fusepb.FileInfo) []fuse.Dirent {
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

// Dir represents a directory node. It contains a pass by value reference to the grpc client for fetching data.
// It contains a path (including self) of its parent directory
type Dir struct {
	Client    fusepb.FuseControllerClient
	Data      []*fusepb.FileInfo
	Dir       string
	LastFetch time.Time
	mu        sync.Mutex
	*ConnectConfig
}

func (f *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	if err := f.fetchData(); err != nil {
		return nil, err
	}
	for _, d := range f.Data {
		if d.Name == req.Name {
			if d.IsDir {
				return &Dir{Dir: filepath.Join(f.Dir, d.Name), Client: f.Client}, nil
			} else {
				return &File{FileName: filepath.Join(f.Dir, d.Name), Client: f.Client}, nil
			}
		}
	}
	return nil, syscall.ENOENT
}

func (f *Dir) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Inode = 1
	attr.Mode = os.ModeDir | 0o555
	return nil
}

func (f *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	if err := f.fetchData(); err != nil {
		return nil, err
	}
	return ConvertToDirent(f.Data), nil
}

func (f *Dir) fetchData() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if time.Since(f.LastFetch) < 5*time.Second {
		return nil
	}
	r, err := f.Client.FileInfo(context.Background(), &fusepb.FileInfoRequest{Dir: f.Dir})
	if err != nil {
		fmt.Println("err in dirent", err.Error())
		return err
	}
	f.Data = r.Files
	f.LastFetch = time.Now()
	return nil
}

// File represents a single file. It contains the path to itself and the grpc client.
type File struct {
	FileName  string
	Client    fusepb.FuseControllerClient
	Info      *fusepb.FileInfo
	FetchTime time.Time
	mu        sync.Mutex
	*ConnectConfig
}

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	if err := f.getData(); err != nil {
		return err
	}
	res, err := f.Client.FileContent(ctx, &fusepb.RequestContent{
		Offset: uint64(req.Offset),
		Path:   f.FileName,
		Size:   uint64(req.Size),
	})
	if err != nil {
		return err
	}
	resp.Data = res.Content
	return nil
}

func (f *File) Attr(ctx context.Context, attr *fuse.Attr) error {
	if err := f.getData(); err != nil {
		return err
	}
	attr.Inode = 6
	attr.Mode = 0o444
	attr.Size = uint64(f.Info.Size)
	return nil
}

func (f *File) getData() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	res, err := f.Client.SingleFileInfo(context.Background(), &fusepb.SingleFileInfoRequest{Path: f.FileName})
	if err != nil {
		return err
	}
	f.Info = res.Info
	return nil
}
