package machinist

import (
	"encoding/json"
	"github.com/enfabrica/enkit/lib/token"
	machinist "github.com/enfabrica/enkit/machinist/rpc"
	"net"
)

func NewServer() *ServerRequest {
	return &ServerRequest{}
}

type Server struct {
	Config  ServerFlagSet
	Encoder token.BinaryEncoder
}

func (m Server) Poll(server machinist.Controller_PollServer) error {
	panic("implement me")
}

func (m Server) Upload(server machinist.Controller_UploadServer) error {
	panic("implement me")
}

func (m Server) Download(request *machinist.DownloadRequest, server machinist.Controller_DownloadServer) error {
	panic("implement me")
}

type invitationToken struct {
	Addresses []string
	Port      int
}

func (m Server) GenerateInvitation() ([]byte, error) {
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
		Port:      m.Config.Port,
		Addresses: attachedIpAddresses,
	}
	jsonString, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}
	encodedToken, err := m.Encoder.Encode(jsonString)
	if err != nil {
		return nil, err
	}
	return encodedToken, nil
}

type ServerRequest struct {
	Port    int
	Encoder token.BinaryEncoder
}

func (rs *ServerRequest) WithPort(p int) *ServerRequest {
	rs.Port = p
	return rs
}

func (rs ServerRequest) Start() (*Server, error) {
	return &Server{}, nil
}

func (rs ServerRequest) UseEncoder(encoder token.BinaryEncoder) {

}
