//go:build !release
// +build !release

package testserver

import (
	"bytes"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// NewWebsocketCounterServer makes a websocket server that swallows all written responses and executes a callback
func NewWebsocketCounterServer(t *testing.T, onMessage func(input []byte)) *httptest.Server {
	upg := websocket.Upgrader{}
	m := http.NewServeMux()
	m.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		webConn, err := upg.Upgrade(writer, request, nil)
		if err != nil {
			assert.NoError(t, err)
			http.Error(writer, "internal test error", 500)
			return
		}
		for {
			_, data, err := webConn.ReadMessage()
			if err != nil { /* we discard errors here because as of right now there is no way to disconnect gracefully https://github.com/gorilla/websocket/issues/448 */
				return
			}
			onMessage(data)
		}
	})
	return httptest.NewServer(m)
}

// WriteHellosToListener handles writes and reads to net.Listener for testing. For every net.Conn accepted, every write will be responded to with
// `hello <client payload>`
func WriteHellosToListener(t *testing.T, l net.Listener) {
	for {
		c, err := l.Accept()
		assert.NoError(t, err)
		go handleNewHelloNetConn(t, c)
	}
}

// handleNewHelloNetConn accepts the new client webConn and then just writes back "hello <initial payload>"
func handleNewHelloNetConn(t *testing.T, c net.Conn) {
	for {
		buf := make([]byte, 1024)
		_, err := c.Read(buf)
		buf = bytes.Trim(buf, "\x00")
		assert.NoError(t, err)
		retBytes := append([]byte("hello "), buf...)
		_, err = c.Write(retBytes)
		assert.NoError(t, err)
	}
}
