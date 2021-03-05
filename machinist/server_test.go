package machinist_test

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/enfabrica/enkit/machinist"
	"testing"
)

func TestRunServerAndGenerateInvite(t *testing.T)  {
	netAddr, err := ktest.AllocatePort()
	if err != nil {
		t.Fatal(err.Error())
	}
	server, err := machinist.NewServer().
		WithPort(netAddr.Port).
		Start()
	if err != nil {
		t.Fatal(err.Error())
	}
	inviteString, err := server.GenerateInvitation()
	if err != nil {
		t.Error(err.Error())
	}
	fmt.Println(inviteString)
	//debug inviteString
}


func TestJoinNodes(t *testing.T){

}


func TestJoinNodesAndKill(t *testing.T){

}

func TestJoinNodesAndRejoin(t *testing.T){

}
