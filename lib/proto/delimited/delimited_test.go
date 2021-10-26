package delimited

import (
	"io"
	"strings"
	"testing"

	"github.com/enfabrica/enkit/lib/errdiff"

	"github.com/stretchr/testify/assert"
)

func TestReaderNext(t *testing.T) {
	testCases := []struct {
		desc    string
		input   string
		want    []string
		wantErr string
	}{
		{
			desc:  "parses messages correctly",
			input: "\x01A\x05quick\x05brown\x03fox\x00\x05jumps\x04over\x03the\x04lazy\x03dog",
			want:  []string{"A", "quick", "brown", "fox", "", "jumps", "over", "the", "lazy", "dog"},
		},
		{
			desc:    "bad end length",
			input:   "\x01A\x05quick\x05brown\x03fox\x00\x05jumps\x04over\x03the\x04lazy\x04dog",
			wantErr: io.ErrUnexpectedEOF.Error(),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rdr := NewReader(strings.NewReader(tc.input))
			var got []string
			var buf []byte
			var err error
			for buf, err = rdr.Next(); err == nil; buf, err = rdr.Next() {
				got = append(got, string(buf))
			}
			if err == io.EOF {
				err = nil
			}
			errdiff.Check(t, err, tc.wantErr)
			if err != nil {
				return
			}
			assert.Equal(t, tc.want, got)
		})
	}
}
