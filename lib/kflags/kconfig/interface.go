package kconfig

import (
	"github.com/enfabrica/enkit/lib/kflags"
)

type EncodeAs string

const (
	EncodeNone      EncodeAs = "string"    // The value is to be passed as is.
	EncodeFile               = "file"      // The value is to be stored in a file, and the path of the file passed to the flag.
	EncodeHex                = "hex"       // The value is to be hex encoded.
	EncodeBase64             = "base64"    // The value is to be base64 encoded.
	EncodeBase64Url          = "base64url" // The value is to be base64 encoded, using url encoding (avoids / and similar)
	// Empty string "" defaults to EncodeNone.
)

type SourceFrom string

const (
	SourceInline SourceFrom = "inline" // The value represents a string to pass to the env or flag after encoding.
	SourceURL               = "url"    // The value represents an http / https url to retrieve, encode, and pass as env or flag.
	// Empty string "" defaults to inline.
)

type Parameter struct {
	Name  string // What is the name of the parameter?
	Value string // Value is the string value of the parameter.

	Source   SourceFrom // Where to get the value from.
	Encoding EncodeAs   // How to encode the value.

	Hash string // Optional: hash of the value, uesful only when SourceURL is used.
}

type Namespace struct {
	// If empty, it is assumed to be the name of the application.
	// Otherwise, it is a path using "." to separate subcommands.
	Name    string
	Hidden  bool
	Default []Parameter
	Command []Command
}

type Package struct {
	URL  string
	Hash string
}

type Var struct {
	Key, Value string
}

type Implementation struct {
	// Having a Package results in a .tar.gz being downloaded from the specified URL,
	// and in the Manifest contained therein to be loaded.
	Package *Package
	Local   []string
	System  []string

	// Variables to pass to the Local and System commands.
	Var []Var
}

type Manifest struct {
	Command []Command
}

type Command struct {
	kflags.CommandDefinition

	Flag           []kflags.FlagDefinition
	Implementation *Implementation
}

// Goal of a Config is to specify a list of defaults to apply to all the flags in a namespace.
//
// The Config data structure specifies:
//   a) A list of defaults for each namespace (Namespace)
//   b) A list of external config files to fallback to (Include)
//
// When a flag is looked up:
//
// 1) First, the list of current Namespace is looked up. If a match is found, the default is set.
// 2) In order from last to first, all the includes are processed. The default of the first matching include is used.
type Config struct {
	Include   []string
	Namespace []Namespace
}
