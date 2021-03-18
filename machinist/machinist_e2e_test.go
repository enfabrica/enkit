package machinist_test

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/lib/token"
	"github.com/enfabrica/enkit/machinist"
	"github.com/enfabrica/enkit/machinist/mnode"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"net"
	"testing"
	"time"
)



func TestRunServerNodeJoinAndPoll(t *testing.T) {
	descriptor, err := machinist.AllocatePort()
	if err != nil {
		t.Fatal(err.Error())
	}
	rngSeed := rand.New(srand.Source)
	key, err := token.GenerateSymmetricKey(rngSeed, 128)
	assert.Nil(t, err)
	symmetricEncoder, err := token.NewSymmetricEncoder(rngSeed, token.UseSymmetricKey(key))
	assert.Nil(t, err)
	serverRequest := machinist.NewServerRequest().
		UseEncoder(symmetricEncoder).
		WithNetListener(&descriptor.Listener)

	credMod := machinist.WithGenerateNewCredentials(
		kcerts.WithCountries([]string{"US"}),
		kcerts.WithValidUntil(time.Now().AddDate(3, 0, 0)),
		kcerts.WithNotValidBefore(time.Now().Add(-4*time.Minute)),
		kcerts.WithIpAddresses([]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0")}))

	portMod := machinist.WithPortDescriptor(descriptor)
	server, err := machinist.NewServer(serverRequest, credMod, portMod)

	assert.Nil(t, err)
	go t.Run("start machinist master server", func(t *testing.T) {
		if err := server.Start(); err != nil {
			fmt.Println("there was an error")
			t.Fatal(err)
		}
	})
	time.Sleep(2 * time.Second) // Just in case machine is pinned and ipc needs to catch up
	defer server.Close()
	inviteToken, err := server.GenerateInvitation(nil, "node1")
	assert.NoError(t, err)
	////debug inviteString
	node, err := mnode.New(mnode.WithInviteToken(string(inviteToken)))
	if err != nil {
		t.Fatal(err)
	}
	assert.Nil(t, err)
	go node.BeginPolling()
	time.Sleep(2 * time.Second)

	node.Stop()
}
