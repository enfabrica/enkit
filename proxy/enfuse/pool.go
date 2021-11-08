package enfuse

import (
	"errors"
	"github.com/gorilla/websocket"
	"net"
	"sync"
)

var NoServerErr = errors.New("the current server is not set")

// SocketConnectionPool is a simple websocket.Conn pool with the ability to demux from single server connection and multiple clients.
type SocketConnectionPool struct {
	mu           sync.Mutex
	srv          *websocket.Conn
	websocketMap map[string]*websocket.Conn

	srvNet net.Listener
	netLisMap map[string]net.Conn

	prefix int
}

func (scp *SocketConnectionPool) SetServer(conn *websocket.Conn) error {
	scp.mu.Lock()
	defer scp.mu.Unlock()
	scp.srv = conn
	return nil
}

func (scp *SocketConnectionPool) Fetch(m []byte) *websocket.Conn {
	uid := m[:scp.prefix]
	return scp.websocketMap[string(uid)]
}

func (scp *SocketConnectionPool) WriteToServer(msgType int, data []byte, conn *websocket.Conn) error {
	if scp.srv == nil {
		return NoServerErr
	}
	uid := data[:scp.prefix]
	if scp.websocketMap[string(uid)] == nil {
		scp.mu.Lock()
		defer scp.mu.Unlock()
		scp.websocketMap[string(uid)] = conn
	}
	return scp.srv.WriteMessage(msgType, data)
}

func (scp *SocketConnectionPool) WriteToLis(data []byte) error {
	if scp.srv == nil {
		return NoServerErr
	}
	uid := data[:scp.prefix]
	if scp.netLisMap[string(uid)] == nil {
		scp.mu.Lock()
		defer scp.mu.Unlock()
		c, err := net.Dial(scp.srvNet.Addr().Network(), scp.srvNet.Addr().String())
		if err != nil {
			return err
		}
		scp.netLisMap[string(uid)] = c
	}
	_, err := scp.netLisMap[string(uid)].Write(data[scp.prefix:])
	return err
}

func (scp *SocketConnectionPool) ServerPresent() bool {
	return scp.srv != nil
}

func NewPool(strat PayloadAppendStrategy) *SocketConnectionPool {
	id, _ := strat()
	return &SocketConnectionPool{
		prefix:       id,
		websocketMap: map[string]*websocket.Conn{},
	}
}
