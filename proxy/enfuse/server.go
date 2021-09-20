package enfuse

import (
	"context"
	"fmt"
	enfuse "github.com/enfabrica/enkit/proxy/enfuse/rpc"
	"google.golang.org/grpc"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
)

func ServeDirectory(mods ...ServerConfigMod) error {
	s := NewServer(NewServerConfig(mods...))
	return s.Serve()
}

type FuseServer struct {
	cfg *ServerConfig
}

func (s *FuseServer) Files(ctx context.Context, rf *enfuse.RequestFile) (*enfuse.ResponseFile, error) {
	f, err := os.Open(filepath.Join(s.cfg.Dir, rf.Path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data := make([]byte, 1024*1024) // default to sending 1mb at max, client side is 4mb max possible but this is to be safe
	i, err := f.ReadAt(data, int64(rf.Offset))
	if err != nil && err != io.EOF {
		return nil, err
	}
	if i != len(data) {
		data = data[:i]
	}
	return &enfuse.ResponseFile{Content: data}, nil
}

func (s *FuseServer) FileInfo(ctx context.Context, request *enfuse.FileInfoRequest) (*enfuse.FileInfoResponse, error) {
	var dir string
	if request.Dir == "" {
		dir = s.cfg.Dir
	} else {
		dir = filepath.Join(s.cfg.Dir, request.Dir)
	}
	var fis []*enfuse.FileInfo
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		e := &enfuse.FileInfo{
			Name:  info.Name(),
			IsDir: info.IsDir(),
			Size:  info.Size(),
		}
		fis = append(fis, e)
		return nil
	})
	return &enfuse.FileInfoResponse{
		Files: fis,
	}, err
}

func (s *FuseServer) Serve() error {
	grpcs := grpc.NewServer()
	enfuse.RegisterFuseControllerServer(grpcs, s)
	if s.cfg.L == nil {
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.cfg.Url, s.cfg.Port))
		if err != nil {
			return err
		}
		s.cfg.L = l
	}
	return grpcs.Serve(s.cfg.L)
}

func NewServer(cfg *ServerConfig) *FuseServer {
	return &FuseServer{cfg: cfg}
}
