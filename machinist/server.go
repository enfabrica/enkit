package machinist

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/enfabrica/enkit/lib/token"
	machinist "github.com/enfabrica/enkit/machinist/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"log"
	"net"
)

func NewServer(req *ServerRequest, modifiers ...ServerModifier) (*Server, error) {
	s := &Server{
		Encoder: req.Encoder,
	}
	for _, mod := range modifiers {
		if err := mod(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func NewServerRequest() *ServerRequest {
	return &ServerRequest{}
}

func (rs *ServerRequest) WithNetListener(l *net.Listener) *ServerRequest {
	rs.Listener = l
	return rs
}

type Server struct {
	Encoder            token.BinaryEncoder
	NetListener        net.Listener
	NetAddr            *net.TCPAddr
	RunningServer      *grpc.Server
	GenerateServerCert func() (*x509.Certificate, []byte, *rsa.PrivateKey, error)
	FetchTlSConfig     func() (*tls.Config, error)
	CAPem              string
}

func (s *Server) Start() error {
	tlsConfig, err := s.FetchTlSConfig()
	if err != nil {
		return err
	}
	ca := credentials.NewTLS(tlsConfig)
	grpcServer := grpc.NewServer(grpc.Creds(ca))
	machinist.RegisterControllerServer(grpcServer, s)
	s.RunningServer = grpcServer
	return grpcServer.Serve(s.NetListener)
}

func (s *Server) Close() {
	if s.RunningServer == nil {
		panic("cannot call close without first calling start")
	}
	s.RunningServer.Stop()
}

func (s Server) Poll(server machinist.Controller_PollServer) error {
	for {
		_, err := server.Recv()
		if err != nil {
			return err
		}
		response := &machinist.PollResponse{
			Resp: &machinist.PollResponse_Pong{
				Pong: &machinist.ActionPong{

				},
			},
		}
		err = server.Send(response)
		if err != nil {
			log.Println(err.Error())
		}

	}
}

func (s Server) Upload(server machinist.Controller_UploadServer) error {
	panic("implement me")
}

func (s Server) Download(request *machinist.DownloadRequest, server machinist.Controller_DownloadServer) error {
	panic("implement me")
}

type invitationToken struct {
	Addresses  []string
	Port       int
	CRT        string
	PrivateKey string
	RootCA     string
}

func (s Server) GenerateInvitation(tags map[string]string, name string) ([]byte, error) {
	nats, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var attachedIpAddresses []string
	for _, nat := range nats {
		addresses, err := nat.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addresses {
			if tcpNat, ok := addr.(*net.IPNet); ok {
				attachedIpAddresses = append(attachedIpAddresses, tcpNat.IP.String())
			}
			// TODO(adam): not sure if should try and handle non ipAddr interfaces
		}
	}
	_, certPem, certPrivate, err := s.GenerateServerCert()
	if err != nil {
		return nil, err
	}
	privatePemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivate),
	})

	i := invitationToken{
		Port:       s.NetAddr.Port,
		Addresses:  attachedIpAddresses,
		CRT:        string(certPem),
		PrivateKey: string(privatePemBytes),
		RootCA:     s.CAPem,
	}
	jsonString, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}
	fmt.Println("jsons string is", string(jsonString))
	encodedToken := base64.RawStdEncoding.EncodeToString(jsonString)
	fmt.Println("encoded opaque token is", encodedToken)
	return []byte(encodedToken), nil
}

type ServerRequest struct {
	Port         int
	Listener     *net.Listener
	Encoder      token.BinaryEncoder
	CA           *x509.Certificate
	CaPem        []byte
	CaPrivateKey *rsa.PrivateKey
}

func (rs *ServerRequest) WithPort(p int) *ServerRequest {
	rs.Port = p
	return rs
}

func (rs *ServerRequest) UseEncoder(encoder token.BinaryEncoder) *ServerRequest {
	rs.Encoder = encoder
	return rs
}

func (rs *ServerRequest) WithCA(ca *x509.Certificate, pem []byte, key *rsa.PrivateKey) *ServerRequest {
	rs.CA = ca
	rs.CaPem = pem
	rs.CaPrivateKey = key
	return rs
}

type ServerModifier func(server *Server) error

func WithGenerateNewCredentials(certOpts ...kcerts.Modifier) ServerModifier {
	return func(server *Server) error {
		opts, err := kcerts.NewOptions(certOpts...)
		if err != nil {
			return err
		}
		rootCert, rootPem, rootPrivateKey, err := kcerts.GenerateNewCARoot(opts)
		if err != nil {
			return err
		}
		intermediateCert, intermediatePem, intermediatePrivateKey, err := kcerts.GenerateIntermediateCertificate(opts, rootCert, rootPrivateKey)
		if err != nil {
			return err
		}
		server.CAPem = string(rootPem)
		server.GenerateServerCert = func() (*x509.Certificate, []byte, *rsa.PrivateKey, error) {
			return kcerts.GenerateServerKey(opts, intermediateCert, intermediatePrivateKey)
		}
		server.FetchTlSConfig = func() (*tls.Config, error) {
			newPool := x509.NewCertPool()
			newPool.AppendCertsFromPEM(rootPem)
			newPool.AppendCertsFromPEM(intermediatePem)
			_, serverPem, serverPrivateKey, err := server.GenerateServerCert()
			if err != nil {
				return nil, err
			}
			privatePemBytes := pem.EncodeToMemory(&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(serverPrivateKey),
			})
			serverCert, err := tls.X509KeyPair(serverPem, privatePemBytes)
			if err != nil {
				return nil, err
			}
			return &tls.Config{
				//ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{serverCert},
				RootCAs:      newPool,
			}, nil
		}
		return nil
	}
}

func WithPortDescriptor(pd *ktest.PortDescriptor) ServerModifier {
	return func(server *Server) error {
		server.NetListener = pd.Listener
		if pd == nil {
			return fmt.Errorf("the port descriptor is null")
		}
		na, err := pd.Addr()
		if err != nil {
			return err
		}
		server.NetAddr = na
		return nil
	}
}
