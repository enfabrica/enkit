package main

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/server"
	echopb "github.com/enfabrica/enkit/test/example/rpc"
	"github.com/rs/cors"
	"google.golang.org/grpc"
	"net/http"
	"os"
)

type EchoController struct {
}

func (e EchoController) Echo(request *echopb.EchoRequest, echoServer echopb.EchoController_EchoServer) error {
	if err := echoServer.Send(&echopb.EchoResponse{Message: request.Message}); err != nil {
		fmt.Println("err sending echo", err.Error())
	} else {
		fmt.Println("sent echo")
	}
	return nil
}

func main() {
	grpcs := grpc.NewServer()
	os.Setenv("PORT", "8080")
	ec := EchoController{}
	echopb.RegisterEchoControllerServer(grpcs, ec)
	h := cors.AllowAll().Handler(http.NewServeMux())
	server.Run(h, grpcs)
}
