package enfuse

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testClientConn struct {
	Name string
	Conn *websocket.Conn
	Shim socketShim
}

var testConnections []*testClientConn = []*testClientConn{
	{
		Name: "client1",
	},
	{
		Name: "client2",
	},
}

func TestSocketShim(t *testing.T) {
	m := http.NewServeMux()
	upg := websocket.Upgrader{}
	pool := NewPool(DefaultPayloadStrategy)
	m.HandleFunc("/client", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upg.Upgrade(writer, request, nil)
		if err != nil {
			http.Error(writer, "internal test error", 500)
			return
		}
		for {
			m, t, err := conn.ReadMessage()
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			if err := pool.WriteToServer(m, t, conn); err != nil {
				fmt.Println(err.Error())
				continue
			}
		}
	})
	m.HandleFunc("/server", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upg.Upgrade(writer, request, nil)
		if err != nil {
			http.Error(writer, "internal test error", 500)
			return
		}
		if err := pool.SetServer(conn); err != nil {
			http.Error(writer, "server set err ", 500)
			return
		}
		for {
			t, payload, err := conn.ReadMessage()
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			clientConn := pool.Fetch(payload)
			if clientConn != nil {
				if err := clientConn.WriteMessage(t, payload); err != nil {
					fmt.Println(err.Error())
					return
				}
			}
		}
	})
	s := httptest.NewServer(m)
	defer s.Close()
	url := strings.Replace(s.URL, "http", "ws", -1)
	for _, c := range testConnections {
		conn, _, err := websocket.DefaultDialer.Dial(url+"/client", nil)
		assert.NoError(t, err)
		c.Conn = conn
		c.Shim = newShim(DefaultPayloadStrategy, conn)
	}
	conn, _, err := websocket.DefaultDialer.Dial(url+"/server", nil)
	assert.NoError(t, err)
	l, _ := knetwork.AllocatePort()
	dup := NewSocketPayloadDuplex(DefaultPayloadStrategy, conn, l)
	defer dup.Close()
	for _, c := range testConnections {
		_, err := c.Shim.Write([]byte(c.Name))
		assert.NoError(t, err)
	}
	t.Fail()
}

func handleNewConns() {

}

func handleThing() {

}
