package pclient

import (
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/lib/token"
	"github.com/enfabrica/enkit/proxy/nasshp"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"testing"
)

func ReadN(r io.Reader, l int, buffer []byte) (int, error) {
	total := 0
	if len(buffer) < l {
		l = len(buffer)
	}

	for l > 0 {
		got, err := r.Read(buffer)
		if err != nil {
			return total, err
		}
		total += got
		if got > l {
			break
		}
		buffer = buffer[got:]
		l -= got
	}
	return total, nil
}

type Acceptor struct {
	c chan net.Conn
}

func (mc *Acceptor) Get() net.Conn {
	return <-mc.c
}

func (mc *Acceptor) Accept(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("ACCEPT FAILED - %s", err)
			return
		}
		mc.c <- conn
	}

}

func Listener() (int, *Acceptor, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mc := &Acceptor{c: make(chan net.Conn, 16)}
	go mc.Accept(ln)

	return port, mc, err
}

var quote1 = "The world is a dangerous place to live, not because of the people who are evil, but because of the people who don't do anything about it."
var quote2 = "The future depends on what you do today."

func TestBasic(t *testing.T) {
	log.Printf("TEST")
	buffer := [8192]byte{}

	rng := rand.New(srand.Source)
	nassh, err := nasshp.New(rng, nil, nasshp.WithLogging(&logger.DefaultLogger{Printer: log.Printf}),
		nasshp.WithSymmetricOptions(token.WithGeneratedSymmetricKey(0)),
		nasshp.WithOriginChecker(func(r *http.Request) bool { return true }))
	assert.Nil(t, err)

	mux := http.NewServeMux()
	nassh.Register(mux.HandleFunc)

	log.Printf("SERVER")
	tu, err := ktest.Start(&khttp.Dumper{Log: log.Printf, Real: mux})
	assert.Nil(t, err)
	u, err := url.Parse(tu)
	assert.Nil(t, err)

	p := nasshp.NewBufferPool(32) // Small buffers, stress memory and buffer passing.
	tl, err := NewTunnel(p)
	assert.Nil(t, err)

	log.Printf("LISTENER")
	port, a, err := Listener()
	assert.Nil(t, err)

	// Read from tr the data received from the browser.
	tr, pw := io.Pipe()
	// Write into tw to send data to the browser.
	pr, tw := io.Pipe()

	log.Printf("STARTED")
	go tl.KeepConnected(u, "127.0.0.1", (uint16)(port))
	go tl.Receive(pw)
	go tl.Send(pr)

	log.Printf("GETTING")
	tcp := a.Get()
	log.Printf("GOT")

	tw.Write([]byte(quote1))
	log.Printf("TW WRITE DONE")
	r, err := ReadN(tcp, len(quote1), buffer[:])
	assert.Nil(t, err)
	log.Printf("TCP READ DONE")
	assert.Equal(t, quote1, string(buffer[:r]))

	tcp.Write([]byte(quote2))
	log.Printf("TCP WRITE DONE")
	r, err = ReadN(tr, len(quote2), buffer[:])
	assert.Nil(t, err)
	log.Printf("TR READ DONE")
	assert.Equal(t, quote2, string(buffer[:r]))
}
