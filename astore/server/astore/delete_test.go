package astore

import (
	"fmt"
	"os"
	"testing"
)

func TestServer_Delete(t *testing.T) {
	fmt.Println("runnign test here")
	somethingPort, err := os.LookupEnv("SOMETHING_HERE")
	fmt.Println(somethingPort)
	fmt.Println(err)

}
