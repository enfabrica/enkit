package enfuse_test

import (
	"fmt"
	"github.com/enfabrica/enkit/proxy/enfuse"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func makeEchoServer(t *testing.T) *httptest.Server {
	upg := websocket.Upgrader{}
	m := http.NewServeMux()
	m.HandleFunc("/echo", func(writer http.ResponseWriter, request *http.Request) {
		webConn, err := upg.Upgrade(writer, request, nil)
		if err != nil {
			assert.NoError(t, err)
			http.Error(writer, "internal test error", 500)
			return
		}
		for {
			m, data, err := webConn.ReadMessage()
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			assert.NoError(t, webConn.WriteMessage(m, append([]byte("hello"), data...)))
		}
	})
	return httptest.NewServer(m)
}

func TestSanityPool(t *testing.T) {
	p := enfuse.NewPool(enfuse.DefaultPayloadStrategy)
	uidLen, factoryUIDFunc := enfuse.DefaultPayloadStrategy()
	assert.Nil(t, p.Fetch(make([]byte, uidLen)))
	assert.False(t, p.ServerPresent())

	s := makeEchoServer(t)
	defer s.Close()

	url := strings.Replace(s.URL, "http", "ws", -1)
	serverConn, _, err := websocket.DefaultDialer.Dial(url+"/echo", nil)
	assert.NoError(t, err)

	assert.NoError(t, p.SetServer(serverConn))
	assert.True(t, p.ServerPresent())
	go func() {
		for _ = range make([]int, 100){
			go assert.NoError(t, p.SetServer(serverConn))
			go assert.True(t, p.ServerPresent())
		}
	}()
	time.Sleep(2 * time.Second)

	uid, err := factoryUIDFunc()
	assert.NoError(t, err)

	assert.NoError(t, p.WriteWebsocketServer(websocket.BinaryMessage, append(uid, []byte("hello")...), serverConn))
}
