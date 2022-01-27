package kbuildbarn_test

import (
	"github.com/enfabrica/enkit/lib/kbuildbarn"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type BytStreamResult struct {
	ShouldFail   bool
	Url          string
	ExpectedHash string
	ExpectedSize string
}

var TestByteStreamUrlTable = []BytStreamResult{
	{
		Url:          "bytestream://build.local.enfabrica.net:8000/blobs/a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a/465",
		ShouldFail:   false,
		ExpectedHash: "a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a",
		ExpectedSize: "465",
	},
	{
		Url:          "bytestream://build.local.enfabrica.net:8000/a9a664559b4d29ecb70613fad33acfb287f2fa378178e131feaaebb5dafa231a/465",
		ShouldFail:   true,
		ExpectedHash: "",
		ExpectedSize: "",
	},
	{
		Url:          "bytestream://build.local.enfabrica.net:8000/blobs/foo/bar",
		ShouldFail:   false,
		ExpectedHash: "foo",
		ExpectedSize: "bar",
	},
	{
		Url:          "bytestream://build.local.enfabrica.net:8000",
		ShouldFail:   true,
		ExpectedHash: "",
		ExpectedSize: "",
	},
	{
		Url:          "bytestream://build.local.enfabrica.net:8000////",
		ShouldFail:   true,
		ExpectedHash: "",
		ExpectedSize: "",
	},
	{
		Url:          "bytestream://build.local.enfabrica.net:8000",
		ShouldFail:   true,
		ExpectedHash: "",
		ExpectedSize: "",
	},
}

func TestByteStreamUrl(t *testing.T) {
	for _, c := range TestByteStreamUrlTable {
		hash, size, err := kbuildbarn.ByteStreamUrl(c.Url)
		if c.ShouldFail {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, c.ExpectedHash, hash)
			assert.Equal(t, c.ExpectedSize, size)
		}
	}
}

func removeHttpScheme(url string) string {
	return strings.ReplaceAll(url, "http://", "")
}

func TestRetryUntilSuccess(t *testing.T) {
	succeedOnDirectoryHandler := http.NewServeMux()
	succeedOnDirectoryHandler.HandleFunc("/blobs/directory/", func(writer http.ResponseWriter, request *http.Request) {
		_, err := writer.Write([]byte("hello world"))
		assert.NoError(t, err)
	})
	alwaysFailHandler := http.NewServeMux()
	alwaysFailHandler.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "my error", http.StatusInternalServerError)
	})
	alwaysFailServer := httptest.NewServer(alwaysFailHandler)
	directorySuccessServer := httptest.NewServer(succeedOnDirectoryHandler)

	failParams := kbuildbarn.NewBuildBarnParams(removeHttpScheme(alwaysFailServer.URL), "", "foo", "bar")
	succeedParams := kbuildbarn.NewBuildBarnParams(removeHttpScheme(directorySuccessServer.URL), "", "foo", "bar")

	resp, err := kbuildbarn.RetryUntilSuccess(succeedParams)
	assert.NoError(t, err)
	assert.Equal(t, "hello world", string(resp))

	_, err = kbuildbarn.RetryUntilSuccess(failParams)
	assert.Error(t, err)

}
