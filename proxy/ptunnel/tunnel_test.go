package ptunnel

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/enfabrica/enkit/lib/errdiff"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/lib/token"
	"github.com/enfabrica/enkit/proxy/nasshp"
	"github.com/enfabrica/enkit/proxy/utils"

	"github.com/stretchr/testify/assert"
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
	nassh.Register(mux.Handle)

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

var (
	hosts     = []string{"google.com", "amazon.com", "reddit.com"}
	protocols = []string{"udp|", "tcp|", ""}
	ports     = []int{55, 22, 44}

	bannedHosts = []string{"facebook.com", "nvidia.com", "amd.com"}
	bannedPorts = []int{1337, 8080, 4433}
)

func TestHostLookup(t *testing.T) {
	var testList []string
	for _, proto := range protocols {
		for _, host := range hosts {
			ips, err := net.LookupHost(host)
			assert.Nil(t, err)
			assert.GreaterOrEqual(t, len(ips), 1)
			for _, ip := range ips {
				for _, port := range ports {
					hostport := net.JoinHostPort(ip, strconv.Itoa(port))
					hostport = strings.ReplaceAll(hostport, "[", "\\[")
					hostport = strings.ReplaceAll(hostport, "]", "\\]")
					testList = append(testList, strings.Join([]string{proto, hostport}, ""))
				}
			}
		}
	}
	pList, err := utils.NewPatternList(testList)
	assert.Nil(t, err)
	rng := rand.New(srand.Source)
	nassh, err := nasshp.New(rng, nil,
		nasshp.WithLogging(&logger.DefaultLogger{Printer: log.Printf}),
		nasshp.WithSymmetricOptions(token.WithGeneratedSymmetricKey(0)),
		nasshp.WithOriginChecker(func(r *http.Request) bool { return true }),
		nasshp.WithFilter(pList.Allow),
	)
	assert.Nil(t, err)
	m := http.NewServeMux()
	nassh.Register(m.Handle)
	s := httptest.NewServer(m)
	t.Run("Test Host Resolution with Allowed List", func(t *testing.T) {
		for _, h := range hosts {
			for _, p := range ports {
				t.Run(fmt.Sprintf("Connecting to host: %s with port %d", h, p), testCanConnect(s.URL, h, p, false))
			}
		}
	})
	t.Run("Test Fail Host Resolution", func(t *testing.T) {
		for _, h := range bannedHosts {
			for _, p := range bannedPorts {
				t.Run(fmt.Sprintf("Connecting to host: %s with port %d", h, p), testCanConnect(s.URL, h, p, true))
			}
		}
	})
}

func TestTunnelTypeForHost(t *testing.T) {
	testCases := []struct {
		desc    string
		host    string
		want    TunnelType
		wantErr string
	}{
		{
			desc: "no tunnel required for external URL",
			host: "www.enfabrica.net",
			want: TunnelTypeNone,
		},
		{
			desc: "local tunnel for localhost URL",
			host: "anything.local.enfabrica.net",
			want: TunnelTypeLocal,
		},
		{
			desc:    "error for non-existent URL",
			host:    "does.not.exist.enfabrica.net",
			wantErr: "no such host",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got, gotErr := TunnelTypeForHost(tc.host)
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func testCanConnect(serverUrl, host string, port int, shouldFail bool) func(t *testing.T) {
	return func(t *testing.T) {
		u, err := url.ParseRequestURI(serverUrl)
		assert.Nil(t, err, "%s", err)
		u.Path = "/proxy"
		u.RawQuery = url.Values{"host": {host}, "port": {fmt.Sprintf("%d", port)}}.Encode()
		responseString := ""
		err = protocol.Get(u.String(), protocol.Read(protocol.String(&responseString)))
		if shouldFail {
			assert.NotNil(t, err)
			return
		}
		assert.Nil(t, err, "%s", err)
		// TODO(adam): make some nice SID utility functions, right now this just checks that the sid is sent
		assert.GreaterOrEqual(t, len(responseString), 5, "%s", responseString)
	}
}
