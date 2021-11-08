package enfuse

import (
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"net"
	"sync"
)

var _ io.Writer = &socketShim{}

// socketShim is a simple wrapper that implements io.Writer that write the full buffer.
// in the future this could be nicely reconnectable with a buffer window.
type socketShim struct {
	WebConn *websocket.Conn
	prefix  []byte
	Strat   PayloadAppendStrategy
}

func (s socketShim) Write(p []byte) (n int, err error) {
	//websockets always write the full buffer
	return len(p), s.WebConn.WriteMessage(websocket.BinaryMessage, p)
}

func newShim(strat PayloadAppendStrategy, conn *websocket.Conn) socketShim {
	return socketShim{WebConn: conn, Strat: strat}
}

// SocketPayloadDuplex will forward connections from the websocket.Conn to the net.Listener. the way that it forwards the request
// is determined by the PayloadAppendStrategy passed in.
type SocketPayloadDuplex struct {
	pool         *SocketConnectionPool
	mu           sync.Mutex
	prefixLength int
	c            *websocket.Conn
	l            net.Listener
	shutdown     chan struct{}
}

func NewSocketPayloadDuplex(strat PayloadAppendStrategy, c *websocket.Conn, l net.Listener) *SocketPayloadDuplex {
	s, _ := strat()
	dup := &SocketPayloadDuplex{
		pool:         NewPool(strat),
		prefixLength: s,
		c:            c,
		l:            l,
	}
	dup.shutdown = make(chan struct{}, 1)
	go dup.listenForConn()
	return dup
}

func (s *SocketPayloadDuplex) Close() {

}

func (s *SocketPayloadDuplex) listenForConn() {
	lUrl := s.l.Addr().String()
	for {
		m, data, err := s.c.ReadMessage()
		if err != nil {
			fmt.Println("error doing things", err.Error())
			continue
		}
		if err := s.pool.WriteToLis(data); err != nil {

		}
	}
}

func handleNewConn() {

}
