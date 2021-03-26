package main

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/astore/common"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/machinist/mclient/machinist"
	machinist2 "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/exec"
)

func Start(mflags *machinist.Flags) error {
	rng := rand.New(srand.Source)
	conn, err := grpc.Dial("127.0.0.1:8080", grpc.WithInsecure())
	if err != nil {
		return err
	}
	authClient := auth.NewAuthClient(conn)
	machinistClient := machinist2.NewControllerClient(conn)
	pub, priv, err := box.GenerateKey(rng)
	if err != nil {
		return err
	}
	authReq := &auth.AuthenticateRequest{
		Key:    (*pub)[:],
		User:   "adam",
		Domain: "localhost:5443",
	}
	ctx := context.Background()
	authRes, err := authClient.Authenticate(ctx, authReq)
	if err != nil {
		return err
	}
	fmt.Println(authRes.Url)
	browser.OpenURL(authRes.Url)
	treq := &auth.TokenRequest{
		Url: authRes.Url,
	}
	var tres *auth.TokenResponse
	for {
		tres, err = authClient.Token(context.TODO(), treq)
		if err == nil {
			break
		}
	}
	servPub, err := common.KeyFromSlice(authRes.Key[:])
	if err != nil {
		return fmt.Errorf("server provided invalid key - please retry - %s", err)
	}
	nonce, err := common.NonceFromSlice(tres.Nonce)
	if err != nil {
		return fmt.Errorf("server returned invalid nonce, please try again - %s", err)
	}
	decrypted, ok := box.Open(nil, tres.Token, nonce.ToByte(), servPub.ToByte(), priv)
	if !ok {
		return fmt.Errorf("could not decrypt returned token")
	}

	kc := kcookie.New("adam", string(decrypted))
	md := make(map[string][]string)
	md["cookie"] = []string{kc.String()}
	bctx := metadata.NewOutgoingContext(context.Background(), md)

	dl, err := machinistClient.Download(bctx, &machinist2.DownloadRequest{})
	if err != nil {
		return err
	}
	file, err := ioutil.TempFile("/tmp", "ss")
	if err != nil {
		return err
	}
	left, err := file.Write(dl.Key)
	if err != nil || left == 0 {
		return err
	}

	err = ioutil.WriteFile(file.Name()+"-cert.pub", dl.Cert, 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command("ssh-add", file.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(dl.Capublickey)
	if err != nil {
		return err
	}
	err = kcerts.StartSSHAgent()
	if err != nil {
		fmt.Println("error starting ssh agent")
		return err
	}
	err = kcerts.AddSSHCAToClient(pubKey, dl.Cahosts)
	if err != nil {
		return err
	}
	err = cmd.Run()
	if err != nil {
		return err
	}
	sshConfig := &ssh.ClientConfig{
		User:            "drunkengranite",
		Auth:            []ssh.AuthMethod{SSHAgent()},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	sshConn, err := ssh.Dial("tcp", "hello.local:22", sshConfig)
	if err != nil {
		return err
	}
	session, err := sshConn.NewSession()
	if err != nil {
		return fmt.Errorf("Failed to create session: %s", err)
	}
	//modes := ssh.TerminalModes{
	//	ssh.ECHO:          0,     // disable echoing
	//	ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
	//	ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	//}
	//if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
	//	session.Close()
	//	return fmt.Errorf("request for pseudo terminal failed: %s", err)
	//}
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stdin for session: %v", err)
	}
	go io.Copy(stdin, os.Stdin)

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stdout for session: %v", err)
	}
	go io.Copy(os.Stdout, stdout)

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stderr for session: %v", err)
	}
	go io.Copy(os.Stderr, stderr)
	err = session.Run("/bin/bash")
	return err
}

func SSHAgent() ssh.AuthMethod {
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}
	return nil
}

func main() {
	command := &cobra.Command{
		Use:   "machinist",
		Short: "machinist controls the allocation of a machine through an controller",
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return Start(nil)
	}

	kcobra.Run(command)
}
