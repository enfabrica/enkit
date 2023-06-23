// Package qtc exists to trick the Go tooling into adding required packages to
// go.mod, so that generating dependencies doesn't continually try to remove
// these and break the build.
//
// This situation is arising because:
// * a bazel rule depends on a Go binary built from a Go dependency
// * the Go dependency is fetched from the Gazelle generated set
// * the Gazelle set is generated from go.mod
// * go.mod is inferred based on source imports
// * there is no explicit source import of these Go packages
//
// By adding explicit imports here, the issue is resolved.
package qtc

import (
	_ "github.com/valyala/bytebufferpool"
	_ "github.com/valyala/quicktemplate"
)
