package commands

import (
	"testing"

	"github.com/enfabrica/enkit/lib/errdiff"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeListenAddr(t *testing.T) {
	testCases := []struct{
		desc string
		addr string
		wantNet string
		wantAddr string
		wantErr string
	} {
		{
			desc: "port number only",
			addr: "6443",
			wantNet: "tcp",
			wantAddr: ":6443",
		},
		{
			desc: "port with no host",
			addr: ":6443",
			wantNet: "tcp",
			wantAddr: ":6443",
		},
		{
			desc: "port with host",
			addr: "127.0.0.1:6443",
			wantNet: "tcp",
			wantAddr: "127.0.0.1:6443",
		},
		{
			desc: "tcp url",
			addr: "tcp://127.0.0.1:6443",
			wantNet: "tcp",
			wantAddr: "127.0.0.1:6443",
		},
		{
			desc: "unix domain socket",
			addr: "unix:///tmp/some.sock",
			wantNet: "unix",
			wantAddr: "/tmp/some.sock",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func (t *testing.T) {
			gotNet, gotAddr, gotErr := normalizeListenAddr(tc.addr)

			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			assert.Equal(t, gotNet, tc.wantNet)
			assert.Equal(t, gotAddr, tc.wantAddr)
		})
	}
}