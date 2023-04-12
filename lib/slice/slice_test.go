package slice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToSet(t *testing.T) {
	testCases := []struct {
		desc string
		s    []string
		want map[string]struct{}
	}{
		{
			desc: "nil slice",
			s:    nil,
			want: map[string]struct{}{},
		},
		{
			desc: "empty slice",
			s:    []string{},
			want: map[string]struct{}{},
		},
		{
			desc: "small slice",
			s:    []string{"foo", "bar"},
			want: map[string]struct{}{"foo": struct{}{}, "bar": struct{}{}},
		},
		{
			desc: "deduplicates repeats",
			s:    []string{"foo", "bar", "foo", "foo", "bar"},
			want: map[string]struct{}{"foo": struct{}{}, "bar": struct{}{}},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := ToSet(tc.s)

			assert.Equal(t, got, tc.want)
		})
	}
}
