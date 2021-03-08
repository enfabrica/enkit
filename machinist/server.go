package machinist

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/enfabrica/enkit/lib/token"
	machinist "github.com/enfabrica/enkit/machinist/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"log"
	"math/big"
	"net"
	"time"
)

func NewServer(req *ServerRequest) *Server {

	return &Server{
		Encoder:      req.Encoder,
		Port:         req.Port,
		NetListener:  req.Listener,
		CA:           req.CA,
		CAPrivateKey: req.CaPrivateKey,
	}
}

func NewServerRequest() *ServerRequest {
	return &ServerRequest{}
}

func (rs *ServerRequest) WithNetListener(l *net.Listener) *ServerRequest {
	rs.Listener = l
	return rs
}

type Server struct {
	Encoder       token.BinaryEncoder
	Port          int
	NetListener   *net.Listener
	RunningServer *grpc.Server
	CA            *x509.Certificate
	CAPrivateKey  interface{}
}

func (s *Server) Start() error {
	// set up our server certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, s.CA, certPrivateKey.PublicKey, s.CAPrivateKey)
	if err != nil {
		return err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivateKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivateKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivateKey),
	})

	serverCert, err := tls.X509KeyPair(certPEM.Bytes(), certPrivateKeyPEM.Bytes())
	if err != nil {
		fmt.Println("erroring here")
		return err
	}

	ca := credentials.NewServerTLSFromCert(&serverCert)
	grpcServer := grpc.NewServer(grpc.Creds(ca))
	machinist.RegisterControllerServer(grpcServer, s)
	if s.NetListener == nil {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.Port))
		if err != nil {
			return err
		}
		s.NetListener = &lis
	}
	s.RunningServer = grpcServer
	return grpcServer.Serve(*s.NetListener)
}

func (s *Server) Close() {
	if s.RunningServer == nil {
		panic("cannot call close without first calling start")
	}
	s.RunningServer.Stop()
}

func (s Server) Poll(server machinist.Controller_PollServer) error {
	for {
		in, err := server.Recv()
		if err != nil {
			return err
		}
		log.Printf("GOT %#v", in.Req)
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
	Addresses []string
	Port      int
}

func (s Server) GenerateInvitation(tags map[string]string) ([]byte, error) {
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
			//not sure if should try and handle non ipAddr interfaces
		}
	}
	i := invitationToken{
		Port:      s.Port,
		Addresses: attachedIpAddresses,
	}

	jsonString, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}
	encodedToken, err := s.Encoder.Encode(jsonString)
	if err != nil {
		return nil, err
	}
	return encodedToken, nil
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
