package enfuse

import (
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
)

// RedirectServer is a websocket server
type RedirectServer struct {
	Lis           net.Listener
	connectionMap sync.Map
}

func (r *RedirectServer) ListenAndServe() error {
	mux := http.NewServeMux()
	upgrader := websocket.Upgrader{}
	pool := &SocketConnectionPool{}
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			http.Error(writer, "error upgrading", http.StatusInternalServerError)
			return
		}
		if !pool.ServerPresent() {
			if err := pool.SetServer(conn); err != nil {
				log.Printf(err.Error())
			}
			for {
				//m, t, err := conn.ReadMessage()
				//if err != nil {
				//	log.Printf(err.Error())
				//	continue
				//}
			}
		} else {
			for {
				fmt.Println("waiting for client read")
				t, m, err := conn.ReadMessage()
				fmt.Println("got client read")
				if err != nil {
					log.Println("err reading from conn")
					continue
				}
				fmt.Println("writing to server", m)
				if err := pool.WriteToServer(t, m, conn); err != nil {
					log.Printf("err writing to server %s \n", err)
				}

			}
		}
	})
	return http.Serve(r.Lis, mux)
}

const (
	ModeServer = iota
	ModeClient
)

// RedirectClient converts any messages sent to the net client as a binary dataframe over websocket.
// it is full duplex. You can pass it in over any connection that requires
type RedirectClient struct {
	Name      string
	WebClient *websocket.Conn
	// This is the listener that sits between the websocket connection and the application.
	//
	ProxiedListener net.Listener
	// Valid values are ModeServer or ModeClient. This determines the structure of the network
	// ModeClient: grpcClient -> net.Dial -> net.Listen -> RedirectServer
	// ModeServer: RedirectServer -> net.Dial -> net.Listen -> grpcServer
	Mode     int
	RelayUrl string
	shutdown chan struct{}
}

func (r *RedirectClient) Listen() error {
	webConn, _, err := websocket.DefaultDialer.Dial(r.RelayUrl, nil)
	if err != nil {
		return err
	}
	retChan := make(chan error)
	r.WebClient = webConn
	readShutdown := make(chan struct{}, 2)
	writeShutdown := make(chan struct{}, 2)
	if r.Mode == ModeClient {
		go func() {
			for {
				l, err := r.ProxiedListener.Accept()
				if err != nil {
					log.Println("err accepting", err.Error())
					continue
				}
				go HandleWriter(r.Name, l, r.WebClient, writeShutdown)
				go HandleReads(r.Name, r.WebClient, l, readShutdown)
			}
		}()
	}
	if r.Mode == ModeServer {
		listenerUrl := r.ProxiedListener.Addr().String()
		proxyConn, err := net.Dial("tcp", listenerUrl)
		if err != nil {
			return err
		}
		go HandleWriter(r.Name, proxyConn, r.WebClient, writeShutdown)
		go HandleReads(r.Name, r.WebClient, proxyConn, readShutdown)
	}
	return <-retChan
}

func HandleReads(name string, src *websocket.Conn, dst net.Conn, shutdown chan struct{}) <-chan error {
	retErr := make(chan error, 1)
	go func() {
		for {
			t, reader, err := src.NextReader()
			if err != nil {
				retErr <- err
				return
			}
			if t == websocket.BinaryMessage {
				_, err := io.Copy(dst, reader)
				if err != nil {
					retErr <- err
					return
				}
			}
		}
	}()
	return retErr
}

func HandleWriter(name string, src net.Conn, dst *websocket.Conn, showdown chan struct{}) <-chan error {
	retErr := make(chan error, 1)
	go func() {
		_, err := io.Copy(&socketShim{dst, nil, DefaultPayloadStrategy}, src)
		retErr <- err
	}()
	return retErr
}
