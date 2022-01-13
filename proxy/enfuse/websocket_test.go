package enfuse_test

import (
	"github.com/enfabrica/enkit/proxy/enfuse"
	"github.com/enfabrica/enkit/proxy/enfuse/testserver"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"strings"
	"sync"
	"testing"
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
	wg := sync.WaitGroup{}
	s := testserver.NewWebsocketCounterServer(t, func(input []byte) {
		recvMessages = append(recvMessages, string(input))
		wg.Done()
	})

	url := strings.Replace(s.URL, "http", "ws", -1)
	serverConn, _, err := websocket.DefaultDialer.Dial(url, nil)

	assert.NoError(t, err)
	assert.NoError(t, p.SetServer(serverConn))
	p.WaitForServerSet()
	assert.True(t, p.ServerPresent())
	messages := []string{"winnie", "piglet", "eeyore", "tigger"}
	for _, message := range messages {
		wg.Add(1)
		cc, _, err := websocket.DefaultDialer.Dial(url, nil)
		assert.NoError(t, err)
		err = p.WriteWebsocketServer(websocket.BinaryMessage, []byte(message), enfuse.NewWebsocketLock(cc))
		assert.NoError(t, err)
		assert.NoError(t, cc.Close())
	}
	assert.NoError(t, serverConn.Close())
	// These locks look silly, but it's to prevent race condition detecion with go's test suites,
	// which is important to be on for the main package
	wg.Wait()
	assert.ElementsMatch(t, messages, recvMessages)
	s.Close()
}


func TestClientFetch(t *testing.T) {
	wg := &sync.WaitGroup{}
	s := testserver.NewWebsocketCounterServer(t, func(input []byte) {
		wg.Done()
	})
	url := strings.Replace(s.URL, "http", "ws", -1)
	strategy := testserver.DoubleDigitPayloadStrategy
	p := enfuse.NewPool(strategy)
	serverConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	assert.NoError(t, err)
	assert.NoError(t, p.SetServer(serverConn))
	p.WaitForServerSet()
	clientConns := make([]*enfuse.WebsocketLocker, 5)
	messages := []string{"winnie", "piglet", "eeyore", "tigger"}

	for i, message := range messages {
		wg.Add(1)
		cc, _, err := websocket.DefaultDialer.Dial(url, nil)
		assert.NoError(t, err)
		ccLocker := enfuse.NewWebsocketLock(cc)
		err = p.WriteWebsocketServer(websocket.BinaryMessage, []byte(message), ccLocker)
		assert.NoError(t, err)
		clientConns[i] = ccLocker
	}
	wg.Wait()
	for i, message := range messages {
		assert.Equal(t, clientConns[i], p.Fetch([]byte(message)))
	}
	s.Close()

}
