package bazel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPatterns(t *testing.T) {
	testCases := []struct {
		desc      string
		pattern   Pattern
		target    string
		wantMatch bool
	}{
		{
			desc:      "matching recursive pattern",
			pattern:   RecursivePattern("//foo/bar/"),
			target:    "//foo/bar/baz:quux",
			wantMatch: true,
		},
		{
			desc:      "matching recursive pattern top-level target",
			pattern:   RecursivePattern("//foo/bar/"),
			target:    "//foo/bar:quux",
			wantMatch: true,
		},
		{
			desc:      "non-matching recursive pattern similar dir",
			pattern:   RecursivePattern("//foo/bar/"),
			target:    "//foo/barber:quux",
			wantMatch: false,
		},
		{
			desc:      "matching exact pattern",
			pattern:   ExactPattern("//foo/bar:baz"),
			target:    "//foo/bar:baz",
			wantMatch: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gotMatch := tc.pattern.Contains(tc.target)

			assert.Equal(t, tc.wantMatch, gotMatch)
		})
	}
}

func TestPatternSet(t *testing.T) {
	testCases := []struct {
		desc      string
		patterns  PatternSet
		target    string
		wantMatch bool
	}{
		{
			desc: "matches one pattern",
			patterns: []Pattern{
				RecursivePattern("//foo/bar/"),
			},
			target:    "//foo/bar/baz:quux",
			wantMatch: true,
		},
		{
			desc: "matches no patterns",
			patterns: []Pattern{
				RecursivePattern("//foo/bar/"),
			},
			target:    "//foo/barber:quux",
			wantMatch: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gotMatch := tc.patterns.Contains(tc.target)

			assert.Equal(t, tc.wantMatch, gotMatch)
		})
	}
}
