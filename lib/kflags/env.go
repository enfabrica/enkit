package kflags

import (
	"os"
	"regexp"
	"strings"
)

// An VarMangler is a function capable of turning a set of strings in the name of a variable.
//
// Normally, VarMangler is called with a prefix configured with a function like
// NewEnvAugmenter, the namespace, and the flag name.  It is possible that
// multiple namespaces may be passed.
//
// If the empty string is returned, the variable is not looked up.
// This can be used to prevent some variables from being looked up in the environment, for example.
type VarMangler func(components ...string) string

// A VarRewriter is just like a VarMangler, but does not merge the strings together,
// and works on an element at a time.
//
// Some VarManglers can combine multiple VarRewriters together.
type VarRewriter func(string) string

type EnvAugmenter struct {
	prefix  []string
	mangler VarMangler
}

type EnvModifier func(e *EnvAugmenter)

type EnvModifiers []EnvModifier

func (ems EnvModifiers) Apply(e *EnvAugmenter) {
	for _, em := range ems {
		em(e)
	}
}

// JoinRemap returns a VarMangler that joins each element after passing it through
// the specified rewriters.
//
// A nil rewriter is accepted, and performs no operation.
func JoinRemap(separator string, rewriter ...VarRewriter) VarMangler {
	return func (elements ...string) string {
		result := []string{}
		for _, el := range elements {
			for _, r := range rewriter {
				if r == nil {
					continue
				}

				el = r(el)
			}
			result = append(result, el)
		}

		return strings.Join(result, separator)
	}
}

// Regex defining which characters are considered separators when CamelRewriting.
var ToCamel = regexp.MustCompile(`[^a-zA-Z0-9]`)

// CamelRewrite is a simple VarRewriter that turns a string like "max-network-latency"
// or "max_network_latency" in camel case, MaxNetworkLatency.
func CamelRewrite(el string) string {
	var b strings.Builder
	for _, fragment := range ToCamel.Split(el, -1) {
		b.WriteString(strings.Title(fragment))
	}
	return b.String()
}

// Regex defining which characters should be replaced with _ by UnderscoreRewrite.
var ToUnderscore = regexp.MustCompile(`[^a-zA-Z0-9]`)

// UnderscoreRewrite is a simple VarRewriter that turns unknown chars into _.
//
// It simply replaces all invalid characters (defined by the ToUnderscore
// regexp) with underscores.
func UnderscoreRewrite(el string) string {
	return ToUnderscore.ReplaceAllString(el, "_")
}

// UppercaseRewrite is a simple VarRewriter that upper cases everything.
var UppercaseRewrite = strings.ToUpper;

// The set of remappers used to turn flags into environment variable names.
var DefaultEnvRemap = JoinRemap("_", UnderscoreRewrite, UppercaseRewrite)

// PrefixRemap returns a VarMangler that always prepends the specified prefixes.
func PrefixRemap(mangler VarMangler, prefix ...string) VarMangler {
	return func(elements ...string) string {
		return mangler(append(prefix, elements...)...)
	}
}

// SkipNamespaceRemap returns a VarMangler that ignores the namespace fragment.
func SkipNamespaceRemap(mangler VarMangler) VarMangler {
	return func(elements ...string) string {
		return mangler(elements[1:]...)
	}
}

// WithEnvMangler specifies the VarMangler to turn the name of a flag into an
// the name of an environment variable.
func WithEnvMangler(m VarMangler) EnvModifier {
	return func(e *EnvAugmenter) {
		e.mangler = m
	}
}

// WithPrefixes prepends the specified prefixes to the looked up environment variables.
//
// For example, if VarMangler would normally look up the environment variable ENKIT_KFLAGS_DNS,
// WithPrefixes("PROD", "AMERICA") would look up PROD_AMERICA_ENKIT_KFLAGS_DNS.
func WithPrefixes(prefix ...string) EnvModifier {
	return func(e *EnvAugmenter) {
		e.prefix = prefix
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
// The DefaultEnvRemap will be used to determine that a variable named EN_PATH
// needs to be looked up in the environment.
func NewEnvAugmenter(mods ...EnvModifier) *EnvAugmenter {
	er := &EnvAugmenter{mangler: DefaultEnvRemap}
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
// The exact name of the environment variable is determined by the VarMangler passed
// using WithMangler to the constructor.
//
// The default VarMangler is DefaultEnvRemap.
func (er *EnvAugmenter) VisitFlag(reqns string, fl Flag) (bool, error) {
	tomangle := append(append([]string{}, er.prefix...), reqns, fl.Name())
	env := er.mangler(tomangle...)
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
