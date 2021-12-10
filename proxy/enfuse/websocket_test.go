package enfuse_test

import (
	"github.com/enfabrica/enkit/proxy/enfuse"
	"github.com/enfabrica/enkit/proxy/enfuse/testserver"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBasicPoolSet(t *testing.T) {
	p := enfuse.NewPool(enfuse.DefaultPayloadStrategy)
	assert.False(t, p.ServerPresent())
	server1 := &websocket.Conn{}
	assert.NoError(t, p.SetServer(server1))
	assert.True(t, p.ServerPresent())
}

func TestFetchReturnNilWithNoClients(t *testing.T) {
	p := enfuse.NewPool(enfuse.DefaultPayloadStrategy)
	uidLen, _ := enfuse.DefaultPayloadStrategy()
	assert.Nil(t, p.Fetch(make([]byte, uidLen)))
}

func TestWritesToServer(t *testing.T) {
	strategy := testserver.DoubleDigitPayloadStrategy
	p := enfuse.NewPool(strategy)
	uidLen, _ := strategy()

	assert.Nil(t, p.Fetch(make([]byte, uidLen)))
	assert.False(t, p.ServerPresent())

	var recvMessages []string
	mu := sync.Mutex{}
	s := testserver.NewWebsocketCounterServer(t, func(input []byte) {
		mu.Lock()
		recvMessages = append(recvMessages, string(input))
		mu.Unlock()
	})
	defer s.Close()

	url := strings.Replace(s.URL, "http", "ws", -1)
	serverConn, _, err := websocket.DefaultDialer.Dial(url, nil)

	assert.NoError(t, err)
	assert.NoError(t, p.SetServer(serverConn))

	assert.True(t, p.ServerPresent())
	messages := []string{"winnie", "piglet", "eeyore", "tigger"}
	for _, message := range messages {
		cc, _, err := websocket.DefaultDialer.Dial(url, nil)
		assert.NoError(t, err)
		err = p.WriteWebsocketServer(websocket.BinaryMessage, []byte(message), enfuse.NewWebsocketLock(cc))
		assert.NoError(t, err)
	}
	// give time for cpu cycles
	time.Sleep(200 * time.Millisecond)
	// These locks look silly, but it's to prevent race condition detecion with go's test suites,
	// which is important to be on for the main package
	mu.Lock()
	assert.ElementsMatch(t, messages, recvMessages)
	mu.Unlock()
}


func TestClientFetch(t *testing.T) {
	s := testserver.NewWebsocketCounterServer(t, func(input []byte) {})
	defer s.Close()
	url := strings.Replace(s.URL, "http", "ws", -1)
	strategy := testserver.DoubleDigitPayloadStrategy
	p := enfuse.NewPool(strategy)
	serverConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	assert.NoError(t, err)
	assert.NoError(t, p.SetServer(serverConn))

	clientConns := make([]*enfuse.WebsocketLocker, 5)
	messages := []string{"winnie", "piglet", "eeyore", "tigger"}

	for i, message := range messages {
		cc, _, err := websocket.DefaultDialer.Dial(url, nil)
		assert.NoError(t, err)
		ccLocker := enfuse.NewWebsocketLock(cc)
		err = p.WriteWebsocketServer(websocket.BinaryMessage, []byte(message), ccLocker)
		assert.NoError(t, err)
		clientConns[i] = ccLocker
	}
	// give time for cpu cycles
	time.Sleep(200 * time.Millisecond)
	for i, message := range messages {
		assert.Equal(t, clientConns[i], p.Fetch([]byte(message)))
	}
}
