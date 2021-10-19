package enfuse

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"log"
	"sync"
)

var NoServerErr = errors.New("the current server is not set")

// SocketConnectionPool is a simple websocket.Conn pool with the ability to demux from single server connection and multiple clients.
type SocketConnectionPool struct {
	mu      sync.Mutex
	srv     *SocketConnection
	clients []*SocketConnection
}

type SocketConnection struct {
	uuid uuid.UUID
	conn *websocket.Conn
}

func (scp *SocketConnectionPool) AddClient(conn *websocket.Conn) {
	scp.mu.Lock()
	defer scp.mu.Unlock()
	scp.clients = append(scp.clients, &SocketConnection{conn: conn, uuid: uuid.New()})
}

func (scp *SocketConnectionPool) SetServer(conn *websocket.Conn) error {
	scp.mu.Lock()
	defer scp.mu.Unlock()
	scp.srv = &SocketConnection{uuid: uuid.New(), conn: conn}
	return nil
}

func (scp *SocketConnectionPool) Fetch(uuid string) (*SocketConnection, error) {
	for _, s := range scp.clients {
		if s.uuid.String() == uuid {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%s was not a connection", uuid)
}

func (scp *SocketConnectionPool) WriteToServer(msgType int, data []byte) error {
	if scp.srv == nil {
		return NoServerErr
	}
	return scp.srv.conn.WriteMessage(msgType, data)
}

func (scp *SocketConnectionPool) WriteToAllClients(msgType int, data []byte) {
	for _, s := range scp.clients {
		if err := s.conn.WriteMessage(msgType, data); err != nil {
			log.Printf("error in writing to all clients", err.Error())
		}
	}
}


func (scp *SocketConnectionPool) ServerPresent() bool  {
	return scp.srv != nil
}