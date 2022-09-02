package commands

import (
	"io/ioutil"
	"testing"

	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/errdiff"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/logger"

	"github.com/stretchr/testify/assert"
)

func TestSSHParseFlags(t *testing.T) {
	testCases := []struct {
		desc         string
		proxyList    []string
		wantProxyMap []proxyMapping
		wantErr      string
	}{
		{
			desc:         "no entries",
			proxyList:    nil,
			wantProxyMap: nil,
		},
		{
			desc:      "one entry",
			proxyList: []string{".foo=https://foo.example.com./"},
			wantProxyMap: []proxyMapping{
				proxyMapping{substring: ".foo", proxy: "https://foo.example.com./"},
			},
		},
		{
			desc:      "multiple entries",
			proxyList: []string{".foo=https://foo.example.com./", ".bar=https://bar.example.com./"},
			wantProxyMap: []proxyMapping{
				proxyMapping{substring: ".foo", proxy: "https://foo.example.com./"},
				proxyMapping{substring: ".bar", proxy: "https://bar.example.com./"},
			},
		},
		{
			desc:      "parse error",
			proxyList: []string{".foo:https://foo.example.com./"},
			wantErr:   "not a valid proxy mapping",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ssh := &SSH{
				proxyList: tc.proxyList,
			}

			gotErr := ssh.parseFlags(nil, nil)

			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			assert.Equal(t, ssh.ProxyMap, tc.wantProxyMap)
		})
	}
}

var exampleProxyMap = []proxyMapping{
	proxyMapping{substring: ".foo", proxy: "https://foo.example.com./"},
	proxyMapping{substring: ".bar", proxy: "https://bar.example.com./"},
	proxyMapping{substring: ".baz", proxy: "https://baz.example.com./"},
}

func TestSSHChooseProxy(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "en")
	assert.Nil(t, err)
	old := kcerts.GetConfigDir
	defer func() { kcerts.GetConfigDir = old }()
	kcerts.GetConfigDir = func(app string, namespaces ...string) (string, error) {
		return tmpDir + "/.config/enkit", nil
	}
	testCases := []struct {
		desc      string
		proxy     string
		proxyMap  []proxyMapping
		sshArgs   []string // One of these is expected to be the target ($USER@$MACHINE)
		wantProxy string   // Flag in the form ` --proxy=$PROXY_URL`
	}{
		{
			desc:      "target in proxy map",
			proxy:     "https://default.example.com./",
			proxyMap:  exampleProxyMap,
			sshArgs:   []string{"user@machine-1.bar"},
			wantProxy: " --proxy=https://bar.example.com./",
		},
		{
			desc:      "target not in proxy map with default proxy",
			proxy:     "https://default.example.com./",
			proxyMap:  exampleProxyMap,
			sshArgs:   []string{"user@machine-1.quux"},
			wantProxy: " --proxy=https://default.example.com./",
		},
		{
			desc:      "target not in proxy map with no default proxy",
			proxyMap:  exampleProxyMap,
			sshArgs:   []string{"user@machine-1.quux"},
			wantProxy: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ssh := &SSH{
				Proxy:    tc.proxy,
				ProxyMap: tc.proxyMap,
				BaseFlags: &client.BaseFlags{
					Log: &logger.Proxy{
						Logger: logger.Nil,
					},
				},
			}

			got := ssh.chooseProxy(tc.sshArgs)

			assert.Equal(t, got, tc.wantProxy)
		})
	}
}
