package nasshp

import (
	"encoding/binary"
	"fmt"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/lib/token"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type Acceptor struct {
	c chan net.Conn
}

func (mc *Acceptor) Get() net.Conn {
	return <-mc.c
}

func (mc *Acceptor) Accept(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("ACCEPT FAILED - %s", err)
			return
		}
		mc.c <- conn
	}

}

func Listener() (int, *Acceptor, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mc := &Acceptor{c: make(chan net.Conn, 16)}
	go mc.Accept(ln)

	return port, mc, err
}

var wisdom = "Nowadays, anyone who wishes to combat lies and ignorance and to write the truth must overcome at least five difficulties. He must have the courage to write the truth when truth is everywhere opposed; the keenness to recognize it, although it is everywhere concealed; the skill to manipulate it as a weapon; the judgment to select those in whose hands it will be effective; and the running to spread the truth among such persons."

func TestBasic(t *testing.T) {
	rng := rand.New(srand.Source)
	nassh, err := New(rng, nil, WithLogging(&logger.DefaultLogger{Printer: t.Logf}),
		WithSymmetricOptions(token.WithGeneratedSymmetricKey(0)),
		WithOriginChecker(func(r *http.Request) bool { return true }))
	assert.Nil(t, err)

	mux := http.NewServeMux()
	nassh.Register(mux.HandleFunc)

	tu, err := ktest.Start(&khttp.Dumper{Log: t.Logf, Real: mux})
	assert.Nil(t, err)
	u, err := url.Parse(tu)
	assert.Nil(t, err)

	// Get authenticated session id first.
	u.Path = "/proxy"

	// Try with invalid parameters first.
	sid := ""
	err = protocol.Get(u.String(), protocol.Read(protocol.String(&sid)))
	assert.NotNil(t, err, "%s", err)

	port, a, err := Listener()
	assert.Nil(t, err)

	// Try again with the correct parameters.
	u.RawQuery = url.Values{"host": {"127.0.0.1"}, "port": {fmt.Sprintf("%d", port)}}.Encode()
	err = protocol.Get(u.String(), protocol.Read(protocol.String(&sid)))
	assert.Nil(t, err, "%s", err)
	assert.NotEqual(t, "", sid)

	// Open the web socket.
	u.Scheme = "ws"
	u.Path = "/connect"
	u.RawQuery = url.Values{"sid": {strings.TrimSpace(sid)}}.Encode()

	c, r, err := websocket.DefaultDialer.Dial(u.String(), nil)
	assert.Nil(t, err)
	assert.NotNil(t, c)
	assert.NotNil(t, r)
	tcp := a.Get()

	err = c.WriteMessage(websocket.BinaryMessage, []byte("\x00\x00\x00\x00"+wisdom))
	assert.Nil(t, err)

	buffer := [8192]byte{}
	data := buffer[:]
	amount, err := tcp.Read(data)
	data = data[:amount]
	assert.Nil(t, err)
	assert.Equal(t, wisdom, string(data))
	amount, err = tcp.Write(data)
	assert.Nil(t, err)
	assert.Equal(t, len(wisdom), amount)

	_, m, err := c.ReadMessage()
	assert.Equal(t, uint32(len(wisdom)), binary.BigEndian.Uint32(m[:4]))
	assert.Equal(t, string(wisdom), string(m[4:]))

	// Acknowledge some bytes, but not all.
	err = c.WriteMessage(websocket.BinaryMessage, []byte("\x00\x00\x00\x07"+wisdom))
	assert.Nil(t, err)

	// The connection now won't write anything. But we should still get an ack back in a little time.
	_, m, err = c.ReadMessage()
	assert.Equal(t, 4, len(m))
	assert.Equal(t, uint32(len(wisdom)*2), binary.BigEndian.Uint32(m[:4]))

	// Close the connection now.
	c.Close()

	// Try to reconnect now, starting from ... somewhere in the past.
	u.RawQuery = url.Values{"sid": {strings.TrimSpace(sid)}, "pos": {fmt.Sprintf("%d", 63)}, "ack": {fmt.Sprintf("%d", 15)}}.Encode()

	c, r, err = websocket.DefaultDialer.Dial(u.String(), nil)
	assert.Nil(t, err)
	assert.NotNil(t, c)
	assert.NotNil(t, r)

	//_, m, err = c.ReadMessage()
	//assert.Equal(t, len(wisdom)-63+4, len(m), "wisdom is %d, resumed from %d", len(wisdom), 63)
	//assert.Equal(t, uint32(0xf), binary.BigEndian.Uint32(m[:4]))
}
