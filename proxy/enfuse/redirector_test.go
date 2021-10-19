package enfuse_test

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/proxy/enfuse"
	"github.com/stretchr/testify/assert"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func initializeClient(t *testing.T, url, name string, mode int) (*enfuse.RedirectClient, net.Listener, chan []byte, chan []byte) {
	writeChan := make(chan []byte)
	readChan := make(chan []byte)
	lis, err := knetwork.AllocatePort()
	assert.NoError(t, err)
	client := &enfuse.RedirectClient{Name: name, RelayUrl: url, ProxiedListener: lis, Mode: mode}
	go func() {
		assert.NoError(t, client.Listen())
	}()
	return client, lis, writeChan, readChan
}

func TestWithNoEncryption(t *testing.T) {
	d, generatedFiles := CreateSeededTmpDir(t, 8)

	// initialize relay server
	redirectServer, err := knetwork.AllocatePort()
	assert.NoError(t, err)
	srv := enfuse.RedirectServer{Lis: redirectServer}
	go func() {
		assert.NoError(t, srv.ListenAndServe())
	}()

	// redirect server url
	tcpAddr, err := redirectServer.Address()
	assert.NoError(t, err)
	serverUrl := "ws://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(tcpAddr.Port)) + "/"

	// initialize redirect client for the server
	_, serverLis, _, _ := initializeClient(t, serverUrl, "serverClient", enfuse.ModeServer)
	s := enfuse.NewServer(
		enfuse.NewServerConfig(
			enfuse.WithDir(d),
			enfuse.WithConnectMods(
				enfuse.WithListener(serverLis),
			),
		),
	)
	go func() {
		assert.NoError(t, s.Serve())
	}()
	time.Sleep(200 * time.Millisecond)

	//test client
	fmt.Println("initializing client client")
	_, redirectClientLis, _, _ := initializeClient(t, serverUrl, "clientClient", enfuse.ModeClient)
	ppp, err := strconv.Atoi(strings.Split(redirectClientLis.Addr().String(), ":")[3])
	assert.NoError(t, err)
	c, err := enfuse.NewClient(&enfuse.ConnectConfig{Port: ppp, Url: "127.0.0.1"})
	assert.NoError(t, err)
	t.Run("Sanity Test Client0", func(t *testing.T) {
		testFile(t, c, generatedFiles)
	})
	c1, err := enfuse.NewClient(&enfuse.ConnectConfig{Port: ppp, Url: "127.0.0.1"})
	assert.NoError(t, err)
	t.Run("Sanity Test Client1", func(t *testing.T) {
		testFile(t, c1, generatedFiles)
	})
	c2, err := enfuse.NewClient(&enfuse.ConnectConfig{Port: ppp, Url: "127.0.0.1"})
	assert.NoError(t, err)
	t.Run("Sanity Test Client2", func(t *testing.T) {
		testFile(t, c2, generatedFiles)
	})
	//t.Fail()
}
