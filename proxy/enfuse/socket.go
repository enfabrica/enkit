package enfuse

import (
	"context"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"time"
)

var (
	_ net.Conn      = &SocketShim{}
	_ io.ReadWriter = &SocketShim{}
)

// SocketShim is a simple wrapper that implements io.ReadWriter that writes and reads the full buffer while translating
// payloads. If read from, it will strip the Prefix from the payload if it is present. If it is written to, it will
// automatically append the Prefix.
// in the future this could be nicely re-connectable with a buffer window.
type SocketShim struct {
	WebConn       *WebsocketLocker
	Prefix        []byte
	PrefixLen     int
	LoggingPrefix string
}

func (s *SocketShim) Read(p []byte) (n int, err error) {
	_, data, err := s.WebConn.ReadMessage()
	if err != nil {
		return 0, err
	}
	realData := data
	copy(p, realData[s.PrefixLen:])
	return len(realData[s.PrefixLen:]), nil
}

func (s *SocketShim) Write(p []byte) (n int, err error) {
	// websockets always write the full buffer
	realPayload := append(s.Prefix, p...)
	return len(p), s.WebConn.WriteMessage(websocket.BinaryMessage, realPayload)
}

func (s *SocketShim) Close() error {
	return s.WebConn.c.Close()
}

func (s *SocketShim) LocalAddr() net.Addr {
	return s.WebConn.c.LocalAddr()
}

func (s *SocketShim) RemoteAddr() net.Addr {
	return s.WebConn.c.RemoteAddr()
}

func (s *SocketShim) SetDeadline(t time.Time) error {
	return multierror.New([]error{
		s.WebConn.c.SetReadDeadline(t),
		s.WebConn.c.SetWriteDeadline(t),
	})
}

func (s *SocketShim) SetReadDeadline(t time.Time) error {
	return s.WebConn.c.SetReadDeadline(t)
}

func (s *SocketShim) SetWriteDeadline(t time.Time) error {
	return s.WebConn.c.SetWriteDeadline(t)
}

func (s *SocketShim) DialOpt() grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, uri string) (net.Conn, error) {
		return s, nil
	})
}

func NewSocketShim(strategy PayloadAppendStrategy, WebLocker *WebsocketLocker) (*SocketShim, error) {
	pLen, f := strategy()
	pre, err := f()
	if err != nil {
		return nil, err
	}
	return &SocketShim{PrefixLen: pLen, Prefix: pre, WebConn: WebLocker, LoggingPrefix: "Client"}, nil
}

func NewWebsocketTCPShim(strategy PayloadAppendStrategy, lis net.Listener, web *websocket.Conn) *WebsocketTCPShim {
	l, _ := strategy()
	wShim := &WebsocketTCPShim{
		prefixLen: l,
		netConn:   lis,
		l:         logger.DefaultLogger{Printer: log.Printf},
	}
	go wShim.handleWrites(NewWebsocketLock(web))
	return wShim
}

// WebsocketTCPShim will half duplex between a websocket connection to a tcp listener.
// if the payload being written indicates it is a new client, it will create a new net.Conn via net.Dial
// locally, and reuse that if the payload indicated it is the same client.
type WebsocketTCPShim struct {
	netConn net.Listener

	prefixLen int

	l logger.Logger
}

func (w *WebsocketTCPShim) handleWrites(webConn *WebsocketLocker) {
	clientMap := make(map[string]net.Conn)
	for {
		_, data, err := webConn.ReadMessage()
		if err != nil {
			w.l.Errorf("Error reading from websocket %v", err)
			continue
		}
		uid := data[:w.prefixLen]
		uidCpy := make([]byte, len(uid))
		copy(uidCpy, uid) /* making a copy here of the uid makes it threadsafe to put into handleReads */
		remainingData := data[w.prefixLen:]
		clientConn := clientMap[string(uid)]
		if clientConn == nil {
			clientConn, err = net.Dial(w.netConn.Addr().Network(), w.netConn.Addr().String())
			if err != nil {
				w.l.Warnf("Error dialing to existing listener %v", err)
				continue
			}
			clientMap[string(uid)] = clientConn
			go w.handleReadToWebsocket(clientConn, webConn, uidCpy)
		}
		_, err = clientConn.Write(remainingData)
		if err != nil {
			w.l.Warnf("Error writing to client %v", err)
			return
		}
	}
}

// handleRead will read from the net.Conn and write the full payload to webConn
func (w *WebsocketTCPShim) handleReadToWebsocket(c net.Conn, webConn *WebsocketLocker, uid []byte) {
	socketShim := &SocketShim{WebConn: webConn, Prefix: uid, PrefixLen: len(uid), LoggingPrefix: "Sharing"}
	for {
		if _, err := io.Copy(socketShim, c); err != nil {
			w.l.Errorf("err copying for client %w", err)
			return
		}
	}
}

func (w *WebsocketTCPShim) Close() error {
	return nil
}
