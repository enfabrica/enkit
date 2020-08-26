package downloader

import (
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/khttp/workpool"
	"github.com/stretchr/testify/assert"
	"testing"
)

// This is not actually testing anything in this library.
// It is just a smoke test invoking the protocol library.
func TestSimple(t *testing.T) {
	_, url, err := ktest.StartServer(ktest.HelloHandler)
	assert.Nil(t, err)

	data := ""
	err = protocol.Get(url, protocol.Read(protocol.String(&data)))
	assert.Nil(t, err)
	assert.Equal(t, "hello", data)
}

func TestParallel(t *testing.T) {
	_, url, err := ktest.StartServer(ktest.HelloHandler)
	assert.Nil(t, err)

	downloader, err := New(WithWorkpoolOptions(workpool.WithWorkers(2)))
	assert.Nil(t, err)

	results := make([]string, 10)
	for i := 0; i < 10; i++ {
		downloader.Get(url, protocol.Read(protocol.String(&results[i])), workpool.ErrorIgnore)
	}
	downloader.Wait()
	for i := 0; i < 10; i++ {
		assert.Equal(t, "hello", results[i])
	}
}
