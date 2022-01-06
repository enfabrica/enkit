//go:build !release
// +build !release

package testserver

import (
	"fmt"
	"github.com/enfabrica/enkit/proxy/enfuse"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

// DoubleDigitPayloadStrategy is used for testing, just increments client payloads up to 20
var DoubleDigitPayloadStrategy enfuse.PayloadAppendStrategy = func() (int, func() ([]byte, error)) {
	counter := 1
	return 2, func() ([]byte, error) {
		defer func() {
			counter = counter + 1
		}()
		return []byte(fmt.Sprintf("%02d\n", counter)), nil
	}
}

// NewWebSocketBasicClientServer makes a basic server for the purposes of testing single instance multi-client to single server configurations
// All clients connected via the /client endpoint will read-write to /server, and using enfuse.WebsocketPool wil read-write
// to /client
func NewWebSocketBasicClientServer(t *testing.T) *httptest.Server {
	upg := websocket.Upgrader{}
	pool := enfuse.NewPool(enfuse.DefaultPayloadStrategy)
	m := http.NewServeMux()
	m.HandleFunc("/client", func(writer http.ResponseWriter, request *http.Request) {
		rawWebConn, err := upg.Upgrade(writer, request, nil)
		if err != nil {
			http.Error(writer, "internal test error", 500)
			return
		}
		for {
			webConn := enfuse.NewWebsocketLock(rawWebConn)
			m, t, err := webConn.ReadMessage()
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			if err := pool.WriteWebsocketServer(m, t, webConn); err != nil {
				fmt.Println("error in write to server", err.Error())
				continue
			}
		}
	})
	m.HandleFunc("/server", func(writer http.ResponseWriter, request *http.Request) {
		webConn, err := upg.Upgrade(writer, request, nil)
		if err != nil {
			assert.NoError(t, err)
			http.Error(writer, "internal test error", 500)
			return
		}
		if err := pool.SetServer(webConn); err != nil {
			assert.NoError(t, err)
			http.Error(writer, "server set err ", 500)
			return
		}
		for {
			payloadType, payload, err := webConn.ReadMessage()
			assert.NoError(t, err)
			clientConn := pool.Fetch(payload)
			if clientConn != nil {
				assert.NoError(t, clientConn.WriteMessage(payloadType, payload))
			}
		}
	})
	return httptest.NewServer(m)
}
