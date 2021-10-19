package enfuse

import (
	"github.com/gorilla/websocket"
	"net/http"
	"testing"
)

func TestWebsocketPool(t *testing.T) {
	mux := http.NewServeMux()
	upgrader := websocket.Upgrader{}
	pool := &SocketConnectionPool{}
	mux.HandleFunc("/client", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			http.Error(writer, "err", 500)
			return
		}
		pool.AddClient(conn)
		return
	})
	mux.HandleFunc("/server", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			http.Error(writer, "err", 500)
		}
		pool.SetServer(conn)
		for {

		}
	})

}
