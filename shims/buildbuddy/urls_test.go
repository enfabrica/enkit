package buildbuddy

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
)

var TestUrls = []string{
	"/file/download?filename=model%2Ffunctional%2Fsfa-model%2Flibmodel-queues-memregions.a&bytestream_url=bytestream%3A%2F%2Fbuild.local.enfabrica.net%3A8000%2Fblobs%2Ff16674c20f3a871becfd1e44d343cf2e7afd6fdbb2fc0be5bb4aa58de497ef5d%2F538362&invocation_id=f69b6189-c598-4241-862a-e52125dd12e6",
	"/file/download?filename=hw%2Fcommon%2Fdv%2Fbase%2Fdv_utils_pkg_elab&bytestream_url=bytestream%3A%2F%2Fbuild.local.enfabrica.net%3A8000%2Fblobs%2Fd083637acce47a592e3a99f329603b8f80d6d96b173ba08997744fc53df081ce%2F3365&invocation_id=d2f85a71-ddff-46f8-bb2b-71db65f8e3c9",
}

func TestFetchUrls(t *testing.T) {
	testMux := http.NewServeMux()
	testMux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		_, err := writer.Write([]byte(request.URL.Query().Get(ByteStreamUrlQueryParam)))
		assert.NoError(t, err)
	})
	exampleByteStreamHost := "foo.bar:1337"
	testServer := httptest.NewServer(testMux)
	testServerUrl, err := url.Parse(testServer.URL)
	assert.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(testServerUrl)
	for _, testUrl := range TestUrls {
		resp := httptest.NewRecorder()
		DefaultHandleFunc(proxy, exampleByteStreamHost)(resp, httptest.NewRequest(http.MethodGet, testUrl, nil))
		r, err := ioutil.ReadAll(resp.Body)
		assert.NoError(t, err)
		parsedResp, err := url.Parse(string(r))
		assert.NoError(t, err)
		assert.Equal(t, parsedResp.Host, exampleByteStreamHost)
	}
}
