// +build !release

package atesting

import (
	"bufio"
	"fmt"
	"github.com/enfabrica/enkit/lib/knetwork"
	"log"
	"net"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type MinioDescriptor struct {
	Port uint16
}

// RunMinioServer will spin up a minio serer using the local docker daemon
// it also returns a func to call that will close and destroy the running image
// the port and network bind are determined by docker and returned
func RunMinioServer() (MinioDescriptor, func() error, error) {
	return MinioDescriptor{}, nil, nil
}

type EmulatedDatastoreDescriptor struct {
	Addr *net.TCPAddr
}

func RunEmulatedDatastore() (*EmulatedDatastoreDescriptor, KillAbleProcess, error) {
	portDescriptor, err := knetwork.AllocatePort()
	if err != nil {
		return nil, nil, err
	}
	tcpAddr, err := portDescriptor.Address()
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.Command("gcloud",
		"beta", "emulators", "datastore", "start",
		"--no-store-on-disk",
		fmt.Sprintf("--host-port=127.0.0.1:%d", tcpAddr.Port),
		"--quiet")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	outputStdErrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	err = cmd.Start()
	killFunc := []func(){
		func() {
			fmt.Println("killing emulator datastore")
			if err := cmd.Process.Kill(); err != nil {
				log.Fatalln(fmt.Sprintf("error killing process %d", cmd.Process.Pid))
			}
		}}
	if err != nil {
		return nil, killFunc, err
	}
	datastoreBooted := make(chan bool)
	// TODO(adam): concatenate stdout and stderr?
	// the datastore emulator writes all logs to the error channel for some reason
	scannerErr := bufio.NewScanner(outputStdErrPipe)
	emulatorOutputText := ""
	go func() {
		for scannerErr.Scan() {
			emulatorOutputText += scannerErr.Text()
			if strings.Contains(scannerErr.Text(), "Dev App Server is now running") {
				datastoreBooted <- true
			}
		}
	}()
	select {
	case <-time.After(15 * time.Second):
		return nil, nil, fmt.Errorf("timeout on starting the emulator, output is %v", emulatorOutputText)
	case result := <-datastoreBooted:
		if result {
			return &EmulatedDatastoreDescriptor{
				Addr: tcpAddr,
			}, killFunc, nil
		}
		return nil, killFunc, fmt.Errorf("unable to start emulator, output is %v", emulatorOutputText)
	}
}



type KillAbleProcess []func()

func (k *KillAbleProcess) KillAll() {
	fmt.Println("running kill functions")
	for _, killFunc := range *k {
		fmt.Println("running kill function")
		killFunc()
	}
}

func (k *KillAbleProcess) AddKillable(process KillAbleProcess) {
	*k = append(*k, process...)
}

func (k *KillAbleProcess) Add(p func()){
	*k = append(*k, p)
}
