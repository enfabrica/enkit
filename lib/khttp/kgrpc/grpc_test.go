package kgrpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/grpcwebclient"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/khttp/kclient"
	"github.com/enfabrica/enkit/lib/khttp/kgrpc/testdata/proto"
	"github.com/enfabrica/enkit/lib/khttp/ktls"
	"github.com/enfabrica/enkit/lib/khttp/ktransport"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"testing"
)

type FortuneServer struct {
}

var grpcFortune = "Amidst the mists and fiercest frosts, With barest wrists, and stoutest boasts"
var httpFortune = "He thrusts his fists against the posts and still insists he sees the ghosts"

func (fs *FortuneServer) Ask(ctx context.Context, req *proto.FortuneRequest) (*proto.FortuneResponse, error) {
	return &proto.FortuneResponse{
		Text: grpcFortune,
	}, nil
}

func Capture(protocol, body *string, tls *uint16, capture *string) protocol.ResponseHandler {
	return func(url string, resp *http.Response, err error) error {
		if err != nil {
			return fmt.Errorf("Request failed: %s", err)
		}

		bytes, _ := httputil.DumpResponse(resp, true)
		*capture = string(bytes)

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Invalid status: %d", resp.Status)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("Could not read body: %s", err)
		}

		*protocol = resp.Proto
		*body = string(data)

		if resp.TLS != nil {
			*tls = resp.TLS.Version
		} else {
			*tls = 0
		}
		return nil
	}
}

func TestSimple(t *testing.T) {
	m := http.NewServeMux()
	m.HandleFunc("/endpoint", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, httpFortune)
	})

	grpcs := grpc.NewServer()
	proto.RegisterFortuneServer(grpcs, &FortuneServer{})

	var wg sync.WaitGroup
	var httpa, httpsa *net.TCPAddr
	wg.Add(3)

	// Start a server capable of h2c, https, http, with a gRPC endpoint and fake server certs.
	go func() {
		err := RunServer(m, grpcs, khttp.WithLogger(log.Printf),
			khttp.WithTLSOptions(ktls.WithCertFile("testdata/certs/public.crt", "testdata/certs/private.key")),
			khttp.WithHTTPAddr(":0"), khttp.WithHTTPSAddr(":0"), khttp.WithWaiter(&wg, &httpa, &httpsa))

		assert.NoError(t, err)
	}()

	wg.Wait()

	// All endpoints to test: cleartext or ssl using http, gRPC, gRPC-web.
	httpe := fmt.Sprintf("http://127.0.0.1:%d/endpoint", httpa.Port)
	httpse := fmt.Sprintf("https://127.0.0.1:%d/endpoint", httpsa.Port)

	grpce := fmt.Sprintf("127.0.0.1:%d", httpa.Port)
	grpcse := fmt.Sprintf("127.0.0.1:%d", httpsa.Port)

	grpcwebe := fmt.Sprintf("http://127.0.0.1:%d/", httpa.Port)
	grpcwebse := fmt.Sprintf("https://127.0.0.1:%d/", httpsa.Port)

	// All transports to test:
	//   plain http v1 (httpe, grpcwebe), h2c (httpe, grpce, grpcwebe),
	//   https v1 (httpse, grpcwebse), http2 prior knowledge (httpse, grpcse, grpcwebse),
	//   http2 upgrade over https (httpse, grpcse, grpcwebse)
	httpt, err := ktransport.NewHTTP1()
	assert.NoError(t, err)

	h2ct, err := ktransport.NewH2C()
	assert.NoError(t, err)

	https1t, err := ktransport.NewHTTP1(ktransport.WithTLSOptions(ktls.WithRootCAFile("testdata/certs/public.crt")))
	assert.NoError(t, err)

	http2t, err := ktransport.NewHTTP2(ktransport.WithTLSOptions2(ktls.WithRootCAFile("testdata/certs/public.crt")))
	assert.NoError(t, err)

	httpst, err := ktransport.NewHTTP(ktransport.WithTLSOptions(ktls.WithRootCAFile("testdata/certs/public.crt")), ktransport.WithForceAttemptHTTP2(true))
	assert.NoError(t, err)

	// Test plain HTTP requests first.
	type connType struct {
		Transport   http.RoundTripper
		Endpoint    string
		Description string
		Proto       string
		TLS         bool
	}

	tests := []connType{
		{Transport: httpt, Proto: "HTTP/1.1", Endpoint: httpe, Description: "http over http1"},
		{Transport: h2ct, Proto: "HTTP/2.0", Endpoint: httpe, Description: "http over h2c"},

		{Transport: https1t, Proto: "HTTP/1.1", Endpoint: httpse, TLS: true, Description: "https over https1"},
		{Transport: http2t, Proto: "HTTP/2.0", Endpoint: httpse, TLS: true, Description: "https over http2 prior knowledge"},
		{Transport: httpst, Proto: "HTTP/2.0", Endpoint: httpse, TLS: true, Description: "https over https/http2 upgrade"},
	}

	for _, test := range tests {
		var proto, body, resp string
		var tver uint16

		err := protocol.Get(test.Endpoint, Capture(&proto, &body, &tver, &resp), protocol.WithClientOptions(
			kclient.WithTransport(test.Transport)))
		assert.NoError(t, err, "failed: %s\n%s", test.Description, resp)
		assert.Equal(t, test.Proto, proto, "failed: %s\n%s", test.Description, resp)
		assert.Equal(t, httpFortune, body, "failed: %s\n%s", test.Description, resp)
		if test.TLS {
			assert.Less(t, uint16(tls.VersionSSL30), tver, "failed: %s\n%s", test.Description, resp)
		} else {
			assert.Equal(t, uint16(0), tver, "failed: %s\n%s", test.Description, resp)
		}
	}

	// Test grpc/grpcweb requests next. Where possible, shares the transport.
	grpch2cc, err := client.Connect(grpce, client.WithInsecure())
	assert.NoError(t, err)

	tlconfig, err := ktls.NewConfig(ktls.WithRootCAFile("testdata/certs/public.crt"))
	assert.NoError(t, err)
	grpchttpsc, err := client.Connect(grpcse, client.GwcOrGrpcOptions{
		grpc.WithTransportCredentials(credentials.NewTLS(tlconfig)),
		gwc.WithHttpSettings(kclient.WithTransport(https1t)),
	})
	assert.NoError(t, err)

	webc, err := client.Connect(grpcwebe, client.WithInsecure())
	assert.NoError(t, err)
	webhttps1c, err := client.Connect(grpcwebse, client.GwcOrGrpcOptions{gwc.WithHttpSettings(kclient.WithTransport(https1t))})
	assert.NoError(t, err)
	webhttp2c, err := client.Connect(grpcwebse, client.GwcOrGrpcOptions{gwc.WithHttpSettings(kclient.WithTransport(http2t))})
	assert.NoError(t, err)
	webhttpsc, err := client.Connect(grpcwebse, client.GwcOrGrpcOptions{gwc.WithHttpSettings(kclient.WithTransport(httpst))})
	assert.NoError(t, err)

	type grpcType struct {
		Conn        grpc.ClientConnInterface
		Description string
	}

	gtests := []grpcType{
		{Conn: grpch2cc, Description: "grpc over h2c"},
		{Conn: grpchttpsc, Description: "grpc over https"},

		{Conn: webc, Description: "grpcweb over http"},
		{Conn: webhttps1c, Description: "grpcweb over https1"},
		{Conn: webhttp2c, Description: "grpcweb over http2 prior knowledge"},
		{Conn: webhttpsc, Description: "grpcweb over https"},
	}

	for _, test := range gtests {
		client := proto.NewFortuneClient(test.Conn)

		resp, err := client.Ask(context.TODO(), &proto.FortuneRequest{})
		assert.NoError(t, err, "grpc resp failed: %s", test.Description)
		if resp != nil {
			assert.Equal(t, grpcFortune, resp.Text)
		}
	}
}
