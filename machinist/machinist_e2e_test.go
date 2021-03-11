package machinist_test

import (
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/lib/token"
	"github.com/enfabrica/enkit/machinist"
	"math/rand"
	"testing"
	"time"
)

func TestRunServerAndGenerateInvite(t *testing.T) {
	descriptor, err := ktest.AllocatePort()
	if err != nil {
		t.Fatal(err.Error())
	}
	//descriptorAddr, err := descriptor.Addr()
	//if err != nil {
	//	t.Fatal(err.Error())
	//}
	rngSeed := rand.New(srand.Source)
	key, err := token.GenerateSymmetricKey(rngSeed, 128)
	if err != nil {
		t.Fatal(err.Error())
	}
	symmetricEncoder, err := token.NewSymmetricEncoder(rngSeed, token.UseSymmetricKey(key))
	if err != nil {
		t.Fatal(err.Error())
	}
	ca, caPem, caPrivateBytes, err := kcerts.GenerateNewCARoot()
	if err != nil {
		t.Fatal(err)
	}

	serverRequest := machinist.NewServerRequest().
		UseEncoder(symmetricEncoder).
		WithNetListener(&descriptor.Listener).
		WithCA(ca, caPem, caPrivateBytes)

	server := machinist.NewServer(serverRequest)
	go t.Run("start machinist master server", func(t *testing.T) {
		if err := server.Start(); err != nil {
			t.Error(err.Error())
		}
	})
	time.Sleep(2 * time.Second)
	defer server.Close()

	//inviteString, err := server.GenerateInvitation(nil)
	//if err != nil {
	//	t.Error(err.Error())
	//}

	//debug inviteString

	//conn, err := grpc.Dial(fmt.Sprintf(":%d", descriptorAddr.Port), grpc.WithInsecure())
	//if err != nil {
	//	t.Fatal(err.Error())
	//}
	//client := machinist2.NewControllerClient(conn)
	//ctx := context.Background()
	//pollStream, err := client.Poll(ctx)
	//if err != nil {
	//	t.Fatal(err.Error())
	//}
	//for {
	//	fmt.Println("polling now")
	//	pollResponse, err := pollStream.Recv()
	//	if err != nil {
	//		t.Error(err)
	//	}
	//	fmt.Println("poll response is %v", pollResponse)
	//	break
	//}

}
