package enfuse

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"context"
	enfuse "github.com/enfabrica/enkit/proxy/enfuse/rpc"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	_ fs.NodeRequestLookuper = &FuseDir{}
	_ fs.Node                = &FuseDir{}

	_ fs.Node         = &FuseFile{}
	_ fs.HandleReader = &FuseFile{}
)

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

// FuseDir represents a directory node. It contains a pass by value reference to the grpc client for fetching data.
// It contains a path (including self) of its parent directory
type FuseDir struct {
	Client    enfuse.FuseControllerClient
	Data      []*enfuse.FileInfo
	Dir       string
	LastFetch time.Time
	sync.Mutex
}

func (f *FuseDir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	f.Lock()
	defer f.Unlock()
	if err := f.fetchData(); err != nil {
		return nil, err
	}
	for _, d := range f.Data {
		if d.Name == req.Name {
			if d.IsDir {
				return &FuseDir{Dir: filepath.Join(f.Dir, d.Name), Client: f.Client}, nil
			} else {
				return &FuseFile{FileName: filepath.Join(f.Dir, d.Name), Client: f.Client}, nil
			}
		}
	}
	return nil, syscall.ENOENT
}

func (f *FuseDir) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Inode = 1
	attr.Mode = os.ModeDir | 0o555
	return nil
}

func (f *FuseDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	f.Lock()
	defer f.Unlock()
	if err := f.fetchData(); err != nil {
		return nil, err
	}
	return ConvertToDirent(f.Data), nil
}

func (f *FuseDir) fetchData() error {
	if time.Since(f.LastFetch) < 5*time.Second {
		return nil
	}
	r, err := f.Client.FileInfo(context.Background(), &enfuse.FileInfoRequest{Dir: f.Dir})
	if err != nil {
		return err
	}
	f.Data = r.Files
	f.LastFetch = time.Now()
	return nil
}

// FuseFile represents a single file. It contains the path to itself and the grpc client.
type FuseFile struct {
	FileName  string
	Client    enfuse.FuseControllerClient
	Info      *enfuse.FileInfo
	FetchTime time.Time
	sync.Mutex
}

func (f *FuseFile) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	f.Lock()
	defer f.Unlock()
	if err := f.getData(); err != nil {
		return err
	}
	res, err := f.Client.Files(ctx, &enfuse.RequestFile{
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

func (f *FuseFile) Attr(ctx context.Context, attr *fuse.Attr) error {
	f.Lock()
	defer f.Unlock()
	if err := f.getData(); err != nil {
		return err
	}
	attr.Inode = 6
	attr.Mode = 0o444
	attr.Size = uint64(f.Info.Size)
	return nil
}

func (f *FuseFile) getData() error {
	res, err := f.Client.SingleFileInfo(context.Background(), &enfuse.SingleFileInfoRequest{Path: f.FileName})
	if err != nil {
		return err
	}
	f.Info = res.Info
	return nil
}
