package enfuse

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/enfabrica/enkit/lib/kcerts"
	fusepb "github.com/enfabrica/enkit/proxy/enfuse/rpc"
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

var _ fusepb.FuseControllerServer = &FuseServer{}

type FuseServer struct {
	cfg *ServerConfig
}

func (s *FuseServer) SingleFileInfo(ctx context.Context, request *fusepb.SingleFileInfoRequest) (*fusepb.SingleFileInfoResponse, error) {
	des, err := os.Open(filepath.Join(s.cfg.Dir, request.Path))
	if err != nil {
		return nil, err
	}
	defer des.Close()
	st, err := des.Stat()
	if err != nil {
		return nil, err
	}
	return &fusepb.SingleFileInfoResponse{
		Info: &fusepb.FileInfo{
			Name:  filepath.Base(request.Path),
			Size:  st.Size(),
			IsDir: false,
		},
	}, nil
}

func (s *FuseServer) FileContent(ctx context.Context, rf *fusepb.RequestContent) (*fusepb.ResponseContent, error) {
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
	return &fusepb.ResponseContent{Content: data}, nil
}

func (s *FuseServer) FileInfo(ctx context.Context, request *fusepb.FileInfoRequest) (*fusepb.FileInfoResponse, error) {
	var dir string
	if request.Dir == "" {
		dir = s.cfg.Dir
	} else {
		dir = filepath.Join(s.cfg.Dir, request.Dir)
	}
	var fis []*fusepb.FileInfo
	outs, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, info := range outs {
		e := &fusepb.FileInfo{
			Name:  info.Name(),
			IsDir: info.IsDir(),
			Size:  info.Size(),
		}
		fis = append(fis, e)
	}
	return &fusepb.FileInfoResponse{
		Files: fis,
	}, err
}

func (s *FuseServer) Serve() error {
	grpcs := grpc.NewServer()
	if s.cfg.ClientInfoChan != nil {
		// Means we are running with mTlS.
		opts, err := kcerts.NewOptions(
			kcerts.WithCountries([]string{"US"}),
			kcerts.WithOrganizations([]string{"Enfabrica"}),
			kcerts.WithValidUntil(time.Now().AddDate(3, 0, 0)),
			kcerts.WithNotValidBefore(time.Now().Add(-10*time.Minute)), // we add this for misconfigured clock wiggle room
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
		rootCertificate, err := tls.X509KeyPair(caPemBytes, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caPk)}))
		if err != nil {
			return err
		}
		rootPool.AppendCertsFromPEM(caPemBytes)
		interPkPemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(interPk)})
		intermediateCertificate, err := tls.X509KeyPair(interPemBytes, interPkPemBytes)
		if err != nil {
			return err
		}
		s.cfg.ClientInfoChan <- &ClientEncryptionInfo{
			CaPublicPem:         caPemBytes,
			IntermediateCertPem: interPemBytes,
			ClientPk:            pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientCertPk)}),
			ClientCertPem:       clientCertPemBytes,
		}
		grpcs = grpc.NewServer(
			grpc.Creds(credentials.NewTLS(&tls.Config{
				Certificates: []tls.Certificate{rootCertificate, intermediateCertificate},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				RootCAs:      rootPool,
				ClientCAs:    rootPool,
			})),
		)
	}
	fusepb.RegisterFuseControllerServer(grpcs, s)
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
