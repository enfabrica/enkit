package enfuse_test

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/proxy/enfuse"
	"github.com/enfabrica/enkit/proxy/enfuse/testserver"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
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
	s := testserver.NewWebSocketBasicClientServer(t)
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
	go testserver.WriteHellosToListener(t, serverNetLis)

	// this will now forward connections to the net.Listener
	serverShim := enfuse.NewWebsocketTCPShim(enfuse.DefaultPayloadStrategy, serverNetLis, serverWebConn)
	defer serverShim.Close()
	for _, c := range testConnections {
		_, err := c.Shim.Write([]byte(c.Name))
		assert.NoError(t, err)
	}
	for _, c := range testConnections {
		buf := make([]byte, 20000)
		numBytesRead, err := c.Shim.Read(buf)
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, fmt.Sprintf("hello %s", c.Name), string(buf[:numBytesRead]))
	}
	for _, c := range testConnections {
		_, err := c.Shim.Write([]byte(c.Name))
		assert.NoError(t, err)
	}
	for _, c := range testConnections {
		buf := make([]byte, 20000)
		numBytesRead, err := c.Shim.Read(buf)
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, fmt.Sprintf("hello %s", c.Name), string(buf[:numBytesRead]))
	}
}