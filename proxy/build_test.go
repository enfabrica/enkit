package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func creator(mapping *Mapping) (http.Handler, error) {
	return NewProxy(mapping.From.Path, mapping.To, mapping.Transform)
}

func TestBuild(t *testing.T) {
	backends := []*httptest.Server{}
	for ix := 0; ix < 10; ix++ {
		proxyId := ix
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "GOT %d:%s", proxyId+1, r.URL.String())
		}))
		defer server.Close()

		backends = append(backends, server)
	}

	mapping := []Mapping{{
		From: HostPath{
			Path: "/host/b1",
		},
		To: backends[0].URL + "/backend1",
	}, {
		From: HostPath{
			Path: "/host/b1/b3",
		},
		To: backends[2].URL + "/backend3",
	},
		{
			From: HostPath{
				Path: "/host/b1/b4/",
			},
			To: backends[3].URL + "/backend4/longer/",
		},
	}

	mux, domains, err := BuildMux(nil, nil, mapping, creator)
	assert.Nil(t, err)
	assert.Equal(t, []string{}, domains)

	proxy := httptest.NewServer(mux)

	get := func(path string) string {
		resp, err := http.Get(proxy.URL + path)
		assert.Nil(t, err)
		body, err := ioutil.ReadAll(resp.Body)
		assert.Nil(t, err)
		return string(body)
	}

	assert.Equal(t, "GOT 1:/backend1", get("/host/b1"))
	assert.Equal(t, "404 page not found\n", get("/host/b1/"))
	assert.Equal(t, "GOT 3:/backend3", get("/host/b1/b3"))
	assert.Equal(t, "GOT 4:/backend4/longer", get("/host/b1/b4"))
	assert.Equal(t, "GOT 4:/backend4/longer/", get("/host/b1/b4/"))
	assert.Equal(t, "GOT 4:/backend4/longer/fuffa", get("/host/b1/b4/fuffa"))
}
