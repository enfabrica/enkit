package kssh

//
//// Listen starts a SSH server listens on given port.
//func GetListener(port int, publicKeys []byte) http.Handler {
//	authorizedKeysMap := map[string]bool{}
//	for len(publicKeys) > 0 {
//		pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(publicKeys)
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		authorizedKeysMap[string(pubKey.Marshal())] = true
//		publicKeys = rest
//	}
//
//	config := &ssh.ServerConfig{
//		PublicKeyCallback: func(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
//			if authorizedKeysMap[string(pubKey.Marshal())] {
//				return &ssh.Permissions{
//					// Record the public key used for authentication.
//					Extensions: map[string]string{
//						"pubkey-fp": ssh.FingerprintSHA256(pubKey),
//					},
//				}, nil
//			}
//			return nil, fmt.Errorf("unknown public key for %q", c.User())
//		},
//	}
//
//	keyPath := filepath.Join(setting.AppDataPath, "ssh/gogs.rsa")
//	if !com.IsExist(keyPath) {
//		os.MkdirAll(filepath.Dir(keyPath), os.ModePerm)
//		_, stderr, err := com.ExecCmd("ssh-keygen", "-f", keyPath, "-t", "rsa", "-N", "")
//		if err != nil {
//			panic(fmt.Sprintf("Fail to generate private key: %v - %s", err, stderr))
//		}
//		log.Trace("New private key is generateed: %s", keyPath)
//	}
//
//	privateBytes, err := ioutil.ReadFile(keyPath)
//	if err != nil {
//		panic("Fail to load private key")
//	}
//	private, err := ssh.ParsePrivateKey(privateBytes)
//	if err != nil {
//		panic("Fail to parse private key")
//	}
//
//	config.AddHostKey(private)
//
//	go listen(config, port)
//}
//
//func listen(config *ssh.ServerConfig, port int) {
//	listener, err := net.Listen("tcp", "0.0.0.0:"+com.ToStr(port))
//	if err != nil {
//		panic(err)
//	}
//	for {
//		// Once a ServerConfig has been configured, connections can be accepted.
//		conn, err := listener.Accept()
//		if err != nil {
//			// handle error
//			continue
//		}
//		// Before use, a handshake must be performed on the incoming net.Conn.
//		sConn, chans, reqs, err := ssh.NewServerConn(conn, config)
//		if err != nil {
//			// handle error
//			continue
//		}
//
//		// The incoming Request channel must be serviced.
//		go ssh.DiscardRequests(reqs)
//		go handleServerConn(sConn.Permissions.Extensions["key-id"], chans)
//	}
//}
//
//func handleServerConn(keyID string, chans <-chan ssh.NewChannel) {
//	for newChan := range chans {
//		if newChan.ChannelType() != "session" {
//			newChan.Reject(ssh.UnknownChannelType, "unknown channel type")
//			continue
//		}
//
//		ch, reqs, err := newChan.Accept()
//		if err != nil {
//			// handle error
//			continue
//		}
//
//		go func(in <-chan *ssh.Request) {
//			defer ch.Close()
//			for req := range in {
//				payload := cleanCommand(string(req.Payload))
//				switch req.Type {
//				case "exec":
//					cmdName := strings.TrimLeft(payload, "'()")
//
//					args := []string{"serv", "key-" + keyID, "--config=" + setting.CustomConf}
//					cmd := exec.Command(setting.AppPath, args...)
//
//					stdout, err := cmd.StdoutPipe()
//					if err != nil {
//						// handle error
//						return
//					}
//					stderr, err := cmd.StderrPipe()
//					if err != nil {
//						// handle error
//						return
//					}
//					input, err := cmd.StdinPipe()
//					if err != nil {
//						// handle error
//						return
//					}
//
//					if err = cmd.Start(); err != nil {
//						// handle error
//						return
//					}
//
//					go io.Copy(input, ch)
//					io.Copy(ch, stdout)
//					io.Copy(ch.Stderr(), stderr)
//
//					if err = cmd.Wait(); err != nil {
//						// handle error
//						return
//					}
//
//					ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
//					return
//				default:
//				}
//			}
//		}(reqs)
//	}
//}
