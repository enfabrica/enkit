package kflags

import (
	"os"
	"regexp"
	"strings"
)

// An EnvMangler is a function capable of turning a set of strings in the name of an environment variable.
//
// Normally, EnvMangler is called with the prefix configured with NewEnvAugmenter, the namespace, and the flag name.
// It is possible that multiple namespaces may be passed.
//
// If the empty string is returned, the env variable is not looked up. This can be used to prevent some
// variables from being looked up in the environment.
type EnvMangler func(components ...string) string

type EnvAugmenter struct {
	prefix  string
	mangler EnvMangler
}

type EnvModifier func(e *EnvAugmenter)

type EnvModifiers []EnvModifier

func (ems EnvModifiers) Apply(e *EnvAugmenter) {
	for _, em := range ems {
		em(e)
	}
}

// Regex defining which characters are expected to be valid in an environment variable name.
var Remapping = regexp.MustCompile(`[^a-zA-Z0-9]`)

// DefaultRemap is a simple EnvMangler that turns a flag name into an env variable.
//
// It simply replaces all invalid characters (defined by the Remapping regexp) with
// underscores, and capitalizes the string.
func DefaultRemap(elements ...string) string {
	els := []string{}
	for _, el := range elements {
		if el == "" {
			continue
		}

		els = append(els, Remapping.ReplaceAllString(el, "_"))
	}
	return strings.ToUpper(strings.Join(els, "_"))
}

// PrefixReamp returns an EnvMangler that always prepends the specified prefixes.
func PrefixRemap(mangler EnvMangler, prefix ...string) EnvMangler {
	return func(elements ...string) string {
		return mangler(append(prefix, elements...)...)
	}
}

// WithMangler specifies the function to use to detrmine the name of the environment variable.
func WithMangler(m EnvMangler) EnvModifier {
	return func(e *EnvAugmenter) {
		e.mangler = m
	}
}

// NewEnvAugmenter creates a new EnvAugmenter.
//
// An EnvAugmenter is an object capable of looking up environment variables to
// pre-populate flag defaults, or change the behavior of your CLI.
//
// The supplied prefix and EnvModifier determine how the lookup is performed.
//
// For example, let's say your CLI expects a flag named 'path', and a prefix
// of 'en' was set, without using WithMangler.
//
// The DefaultRemap will be used to determine that a variable named EN_PATH
// needs to be looked up in the environment.
func NewEnvAugmenter(prefix string, mods ...EnvModifier) *EnvAugmenter {
	er := &EnvAugmenter{
		prefix:  prefix,
		mangler: DefaultRemap,
	}

	EnvModifiers(mods).Apply(er)
	return er
}

// VisitCommand implements the VisitCommand interface of Augmenter. For the EnvAugmenter, it is a noop.
func (er *EnvAugmenter) VisitCommand(reqns string, command Command) (bool, error) {
	return false, nil
}

// VisitFlag implements the VisitFlag interface of Augmenter.
//
// VisitFlag looks for an environment variable named after the configured prefix,
// the requested namespace (reqns) and the flag name.
//
// The exact name of the environment variable is determined by the EnvMangler passed
// using WithMangler to the constructor.
//
// The default EnvMangler is DefaultRemap.
func (er *EnvAugmenter) VisitFlag(reqns string, fl Flag) (bool, error) {
	env := er.mangler(er.prefix, reqns, fl.Name())
	if env == "" {
		return false, nil
	}

	result, found := os.LookupEnv(env)
	if !found {
		return false, nil
	}

	return true, fl.Set(result)
}

// Done implements the Done interface of Augmenter. For the EnvAugmenter, it is a noop.
func (ar *EnvAugmenter) Done() error {
	return nil
}
