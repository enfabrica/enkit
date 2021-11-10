package enfuse_test

import (
	"bytes"
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/proxy/enfuse"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type testClientConn struct {
	Name string
	Conn *websocket.Conn
	Shim *enfuse.SocketShim
}

var testConnections []*testClientConn = []*testClientConn{
	{
		Name: "pluto",
	},
	{
		Name: "donald",
	},
	{
		Name: "mickey",
	},
	{
		Name: "minnie mouse",
	},
	{
		Name: "goofy",
	},
}

func TestSocketShim(t *testing.T) {
	m := creatBasicServer(t)
	s := httptest.NewServer(m)
	defer s.Close()

	// Dial all websocket clients to the httptest.Server
	url := strings.Replace(s.URL, "http", "ws", -1)
	for _, c := range testConnections {
		webConn, _, err := websocket.DefaultDialer.Dial(url+"/client", nil)
		assert.NoError(t, err)
		c.Conn = webConn
		s, err := enfuse.NewSocketShim(enfuse.DefaultPayloadStrategy, webConn)
		assert.NoError(t, err)
		c.Shim = s
	}

	// Setup server
	serverWebConn, _, err := websocket.DefaultDialer.Dial(url+"/server", nil)
	assert.NoError(t, err)
	serverNetLis, _ := knetwork.AllocatePort()

	// since normally this net.Listener is handled by the recv app itself, we implement a basic echo tcp server
	go handleServerLis(t, serverNetLis)
	// this will now forward connections to the net.Listener
	serverShim := enfuse.NewWebsocketTCPShim(enfuse.DefaultPayloadStrategy, serverNetLis, serverWebConn)
	defer serverShim.Close()

	for _, c := range testConnections {
		_, err := c.Shim.Write([]byte(c.Name))
		assert.NoError(t, err)
	}
	for _, c := range testConnections {
		buf := make([]byte, 20000)
		_, err = c.Shim.Read(buf)
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, fmt.Sprintf("hello %s", c.Name), string(bytes.Trim(buf, "\x00")))
	}
	time.Sleep(1 * time.Second)
}

// handleServerLis just spawns a new handleNewServerConn for every new connection
func handleServerLis(t *testing.T, l net.Listener) {
	for {
		c, err := l.Accept()
		assert.NoError(t, err)
		go handleNewServerConn(t, c)
	}
}

// handleNewServerConn accepts the new client webConn and then just writes back "hello <initial payload>"
func handleNewServerConn(t *testing.T, c net.Conn) {
	for {
		buf := make([]byte, 1024)
		_, err := c.Read(buf)
		fmt.Println("after readall", string(buf))
		assert.NoError(t, err)
		retBytes := append([]byte("hello "), buf...)
		_, err = c.Write(retBytes)
		assert.NoError(t, err)
	}
}

// creatBasicServer is a ruddy implementation of a redirect server. It's *really* basic and just serves as a conn forwarder.
func creatBasicServer(t *testing.T) *http.ServeMux {
	fmt.Println("calling create basicServer")
	upg := websocket.Upgrader{}
	pool := enfuse.NewPool(enfuse.DefaultPayloadStrategy)
	m := http.NewServeMux()
	m.HandleFunc("/client", func(writer http.ResponseWriter, request *http.Request) {
		webConn, err := upg.Upgrade(writer, request, nil)
		if err != nil {
			http.Error(writer, "internal test error", 500)
			return
		}
		for {
			m, t, err := webConn.ReadMessage()
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			if err := pool.WriteWebsocketServer(m, t, webConn); err != nil {
				fmt.Println(err.Error())
				continue
			}
		}
	})
	m.HandleFunc("/server", func(writer http.ResponseWriter, request *http.Request) {
		webConn, err := upg.Upgrade(writer, request, nil)
		if err != nil {
			http.Error(writer, "internal test error", 500)
			return
		}
		if err := pool.SetServer(webConn); err != nil {
			http.Error(writer, "server set err ", 500)
			return
		}
		for {
			t, payload, err := webConn.ReadMessage()
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
	return m
}
