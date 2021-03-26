package mserver

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/astore/common"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	authServer "github.com/enfabrica/enkit/astore/server/auth"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/oauth/ogithub"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/lib/token"
	machinist2 "github.com/enfabrica/enkit/machinist"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"net"
	"net/http"
	"reflect"
	"time"
)

func New(modifiers ...ServerModifier) (*Server, error) {
	s := &Server{}
	for _, mod := range modifiers {
		if err := mod(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

type Server struct {
	Encoder             token.BinaryEncoder
	NetListener         net.Listener
	NetAddr             *net.TCPAddr
	RunningServer       *grpc.Server
	PublicSSHCA            []byte
	GenerateServerCert  func() (*x509.Certificate, []byte, *rsa.PrivateKey, error)
	FetchTlSConfig      func() (*tls.Config, error)
	CAPem               string
	IntermediatePrivate *rsa.PrivateKey
	IntermediatePublic  string
	CAPrivateKey        *rsa.PrivateKey
	CASigner            ssh.Signer
}

func (s *Server) Start() error {
	f := &oauth.Flags{
		ExtractorFlags: &oauth.ExtractorFlags{
			LoginTime: time.Hour,
		},
		OauthSecretID:  "c375a13b8ffc37a9bc60",
		OauthSecretKey: "07f3f0b064788d1eae53037c38b40e2d20711f5d",
		TargetURL:      "http://localhost:5443/callback",
		AuthTime:       time.Hour,
	}

	authenticator, err := oauth.New(rand.New(srand.Source),
		oauth.WithFlags(f),
		oauth.WithTargetURL("http://localhost:5443/callback"), ogithub.Defaults())

	rng := rand.New(srand.Source)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer(
		//grpc.StreamInterceptor(ogrpc.StreamInterceptor(authenticator, "/auth.Auth/")),
		//grpc.UnaryInterceptor(ogrpc.UnaryInterceptor(authenticator, "/auth.Auth/")),
	)

	authServer, err := authServer.New(rng, authServer.WithAuthURL("http://localhost:5443/login"), authServer.WithTimeLimit(time.Hour))
	if err != nil {
		return err
	}
	machinist.RegisterControllerServer(grpcServer, s)
	auth.RegisterAuthServer(grpcServer, authServer)

	killChannel := make(chan error)
	s.RunningServer = grpcServer
	fmt.Println("serving grpc server on port", s.NetListener.Addr())
	go func() {
		killChannel <- grpcServer.Serve(s.NetListener)
	}()

	s.CASigner, err = ssh.NewSignerFromKey(s.CAPrivateKey)
	if err != nil {
		return err
	}
	sshpub, err := ssh.NewPublicKey(&s.CAPrivateKey.PublicKey)
	if err != nil {
		return err
	}
	fmt.Println("please add to TrustedUserCAKeys on each host")
	fmt.Println(string(ssh.MarshalAuthorizedKey(sshpub)))

	_, hostPub, err := kcerts.GenerateUserSSHCert(s.CASigner, ssh.HostCert)
	if err != nil {
		return err
	}
	fmt.Println("an example host, please add to host certificate")
	fmt.Println(string(ssh.MarshalAuthorizedKey(hostPub)))

	m := http.NewServeMux()
	m.HandleFunc("/login/", func(w http.ResponseWriter, r *http.Request) {
		key, err := common.KeyFromURL(r.URL.Path)
		if err != nil {
			http.Error(w, "invalid authorization path, tough luck, try again", http.StatusUnauthorized)
			return
		}
		if err := authenticator.PerformLogin(w, r, oauth.WithState(*key), oauth.WithCookieOptions(kcookie.WithPath("/"))); err != nil {
			http.Error(w, "oauth failed, no idea why, ask someone to look at the logs", http.StatusUnauthorized)
			log.Printf("ERROR - could not perform login - %s", err)
			return
		}
	})

	m.HandleFunc("/callback", func(writer http.ResponseWriter, request *http.Request) {
		creds, err := authenticator.ExtractAuth(writer, request)
		if err != nil {
			fmt.Println("error")
		} else {
			fmt.Println(reflect.TypeOf(creds.State))
			if key, ok := creds.State.(common.Key); ok {
				authServer.FeedToken(key, creds.Cookie)
			}
			writer.Write([]byte("delivered credentials"))
		}
	})

	go func() {
		killChannel <- http.ListenAndServe(":5443", m)
	}()

	return <-killChannel
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

func (s Server) Download(ctx context.Context, request *machinist.DownloadRequest) (*machinist.DownloadResponse, error) {
	priv, cert, err := kcerts.GenerateUserSSHCert(s.CASigner, ssh.UserCert)
	if err != nil {
		return nil, err
	}
	pp := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
	return &machinist.DownloadResponse{
		Cert: ssh.MarshalAuthorizedKey(cert),
		Key:  pp,
		Cahosts: []string{"localhost"},
		Capublickey: s.PublicSSHCA,
	}, nil
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

	i := machinist2.InvitationToken{
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
		server.IntermediatePublic = string(intermediatePem)
		server.IntermediatePrivate = intermediatePrivateKey
		server.CAPrivateKey = rootPrivateKey
		pubKey, err := ssh.NewPublicKey(&rootPrivateKey.PublicKey)
		if err != nil {
			return err
		}
		server.PublicSSHCA = ssh.MarshalAuthorizedKey(pubKey)
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

func WithPortDescriptor(pd *PortDescriptor) ServerModifier {
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

func WithEncoder(be token.BinaryEncoder) ServerModifier {
	return func(server *Server) error {
		server.Encoder = be
		return nil
	}
}

type PortDescriptor struct {
	net.Listener
}

func (d PortDescriptor) Addr() (*net.TCPAddr, error) {
	allocatedDatastorePort, ok := d.Listener.Addr().(*net.TCPAddr)
	if !ok {
		return nil, errors.New("shape of the address not correct, is your os not unix")
	}
	return allocatedDatastorePort, nil
}

func AllocatePort() (*PortDescriptor, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}
	return &PortDescriptor{listener}, nil
}

func WithPort(p int) ServerModifier {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
	if err != nil {
		return func(server *Server) error {
			return err
		}
	}
	return WithNetListener(l)
}

func WithNetListener(listener net.Listener) ServerModifier {
	return func(server *Server) error {
		server.NetListener = listener
		return nil
	}
}
