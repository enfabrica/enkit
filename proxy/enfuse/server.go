package enfuse

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/enfabrica/enkit/lib/kcerts"
	enfuse "github.com/enfabrica/enkit/proxy/enfuse/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"time"
)

func ServeDirectory(mods ...ServerConfigMod) error {
	s := NewServer(NewServerConfig(mods...))
	return s.Serve()
}

var _ enfuse.FuseControllerServer = &FuseServer{}

type FuseServer struct {
	cfg *ServerConfig
}

func (s *FuseServer) SingleFileInfo(ctx context.Context, request *enfuse.SingleFileInfoRequest) (*enfuse.SingleFileInfoResponse, error) {
	des, err := os.Open(filepath.Join(s.cfg.Dir, request.Path))
	if err != nil {
		return nil, err
	}
	defer des.Close()
	st, err := des.Stat()
	if err != nil {
		return nil, err
	}
	return &enfuse.SingleFileInfoResponse{
		Info: &enfuse.FileInfo{
			Name:  filepath.Base(request.Path),
			Size:  st.Size(),
			IsDir: false,
		},
	}, nil
}

func (s *FuseServer) Files(ctx context.Context, rf *enfuse.RequestFile) (*enfuse.ResponseFile, error) {
	f, err := os.Open(filepath.Join(s.cfg.Dir, rf.Path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data := make([]byte, rf.Size) // default to sending 1mb at max, client side is 4mb max possible but this is to be safe
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
	outs, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, info := range outs {
		e := &enfuse.FileInfo{
			Name:  info.Name(),
			IsDir: info.IsDir(),
			Size:  info.Size(),
		}
		fis = append(fis, e)
	}
	return &enfuse.FileInfoResponse{
		Files: fis,
	}, err
}

func (s *FuseServer) Serve() error {
	grpcs := grpc.NewServer()
	if s.cfg.ClientInfoChan != nil {
		opts, err := kcerts.NewOptions(
			kcerts.WithCountries([]string{"US"}),
			kcerts.WithOrganizations([]string{"Enfabrica"}),
			kcerts.WithValidUntil(time.Now().AddDate(3, 0, 0)),
			kcerts.WithNotValidBefore(time.Now().Add(-10*time.Minute)),
			kcerts.WithDnsNames(s.cfg.DnsNames),
			kcerts.WithIpAddresses(s.cfg.IpAddresses),
		)
		if err != nil {
			return err
		}
		ca, caPemBytes, caPk, err := kcerts.GenerateNewCARoot(opts)
		if err != nil {
			return err
		}
		_, interPemBytes, interPk, err := kcerts.GenerateIntermediateCertificate(opts, ca, caPk)
		if err != nil {
			return err
		}
		_, clientCertPemBytes, clientCertPk, err := kcerts.GenerateServerKey(opts, ca, caPk)
		if err != nil {
			return err
		}

		rootPool := x509.NewCertPool()
		clientPool := x509.NewCertPool()
		rootPool.AppendCertsFromPEM(caPemBytes)
		clientPool.AppendCertsFromPEM(interPemBytes)

		rootCertificate, err := tls.X509KeyPair(caPemBytes, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caPk)}))
		if err != nil {
			return err
		}
		intermediateCertificate, err := tls.X509KeyPair(interPemBytes, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(interPk)}))
		if err != nil {
			return err
		}
		clientCertificate, err := tls.X509KeyPair(clientCertPemBytes, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientCertPk)}))
		if err != nil {
			return err
		}

		s.cfg.ClientInfoChan <- &ClientInfo{Pool: clientPool, RootPool: rootPool, Certificate: clientCertificate}
		grpcs = grpc.NewServer(
			grpc.Creds(credentials.NewTLS(&tls.Config{
				Certificates: []tls.Certificate{rootCertificate, intermediateCertificate},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				RootCAs:      rootPool,
				ClientCAs:    rootPool,
			})),
		)
	}
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
