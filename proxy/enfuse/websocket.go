package enfuse

import (
	"errors"
	"github.com/gorilla/websocket"
	"sync"
)

// WebsocketLocker is a mutex lock around a websocket since *websocket.Conn isn't threadsafe.
// It has the same function types of a *websocket.Conn
type WebsocketLocker struct {
	c       *websocket.Conn
	writeMu sync.Mutex
	readMu  sync.Mutex
}

func (w *WebsocketLocker) ReadMessage() (messageType int, p []byte, err error) {
	w.readMu.Lock()
	defer w.readMu.Unlock()
	return w.c.ReadMessage()
}

func (w *WebsocketLocker) WriteMessage(messageType int, payload []byte) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.c.WriteMessage(messageType, payload)
}

func NewWebsocketLock(c *websocket.Conn) *WebsocketLocker {
	return &WebsocketLocker{
		c: c,
	}
}

var NoServerErr = errors.New("the current server is not set")

// WebsocketPool is a connection pool with the ability to demux from single server connection and multiple clients.
// it can hold
type WebsocketPool struct {
	mu sync.Mutex

	srvWebsocket *WebsocketLocker
	websocketMap map[string]*WebsocketLocker

	prefixLen int
}

func (scp *WebsocketPool) SetServer(conn *websocket.Conn) error {
	scp.mu.Lock()
	defer scp.mu.Unlock()
	scp.srvWebsocket = NewWebsocketLock(conn)
	return nil
}

func (scp *WebsocketPool) Fetch(m []byte) *WebsocketLocker {
	if v, ok := scp.websocketMap[string(m[:scp.prefixLen])]; ok {
		return v
	}
	return nil
}

func (scp *WebsocketPool) WriteWebsocketServer(msgType int, data []byte, conn *websocket.Conn) error {
	scp.mu.Lock()
	defer scp.mu.Unlock()
	if scp.srvWebsocket == nil {
		return NoServerErr
	}
	uid := data[:scp.prefixLen]
	if scp.Fetch(uid) == nil {
		scp.websocketMap[string(uid)] = NewWebsocketLock(conn)
	}
	return scp.srvWebsocket.WriteMessage(msgType, data)
}

func (scp *WebsocketPool) ServerPresent() bool {
	return scp.srvWebsocket != nil
}

func NewPool(strat PayloadAppendStrategy) *WebsocketPool {
	id, _ := strat()
	return &WebsocketPool{
		prefixLen:    id,
		websocketMap: map[string]*WebsocketLocker{},
	}
}
