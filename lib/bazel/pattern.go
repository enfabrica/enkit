package bazel

import (
	"fmt"
	"strings"
)

type Pattern interface {
	Contains(string) bool
}

// RecursivePattern represents a pattern like `//foo/bar/...`, and matches all
// targets under `//foo/bar`.
type RecursivePattern string

// Contains matches target if target is under this RecursivePattern.
func (p RecursivePattern) Contains(target string) bool {
	if i := strings.IndexRune(target, ':'); i > 0 {
		target = target[:i] + "/"
	}
	return strings.HasPrefix(target, string(p))
}

// ExactPattern represents a pattern that is actually an exact label like
// `//foo/bar:baz`, and only matches `//foo/bar:baz`.
type ExactPattern string

// Contains matches target if target is the same as this pattern.
func (p ExactPattern) Contains(target string) bool {
	return target == string(p)
}

// PatternSet represents a collection of patterns.
type PatternSet []Pattern

// NewPatternSet parses and returns a set of patterns for the supplied pattern
// strings.
func NewPatternSet(patternStrs []string) (PatternSet, error) {
	var patterns []Pattern
	for _, str := range patternStrs {
		if strings.HasSuffix(str, "/...") {
			patterns = append(patterns, RecursivePattern(strings.TrimSuffix(str, "...")))
			continue
		}
		if strings.Count(str, ":") == 1 {
			patterns = append(patterns, ExactPattern(str))
			continue
		}
		return nil, fmt.Errorf("failed to parse pattern %q", str)
	}
	return patterns, nil
}

// Contains returns true if any of the patterns in the set match the given
// target.
func (p PatternSet) Contains(target string) bool {
	for _, pattern := range p {
		if pattern.Contains(target) {
			return true
		}
	}
	return false
}
