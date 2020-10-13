package kflags

import (
	"os"
	"regexp"
	"strings"
)

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

var Remapping = regexp.MustCompile(`[^a-zA-Z0-9]`)

func DefaultRemap(elements ...string) string {
	els := []string{}
	for _, el := range elements {
		if el == "" {
			continue
		}

		els = append(els, Remapping.ReplaceAllString(el, "_"))
	}
	return strings.Join(els, "_")
}

func WithMangler(m EnvMangler) EnvModifier {
	return func(e *EnvAugmenter) {
		e.mangler = m
	}
}

func NewEnvAugmenter(prefix string, mods ...EnvModifier) *EnvAugmenter {
	er := &EnvAugmenter{
		prefix:  prefix,
		mangler: DefaultRemap,
	}

	EnvModifiers(mods).Apply(er)
	return er
}

func (er *EnvAugmenter) VisitCommand(command Command) (bool, error) {
	return false, nil
}

// Visit implements the Visit interface of Augmenter.
func (er *EnvAugmenter) VisitFlag(reqns string, fl Flag) (bool, error) {
	env := er.mangler(er.prefix, reqns, fl.Name())
	result, found := os.LookupEnv(env)
	if !found {
		return false, nil
	}

	return true, fl.Set(result)
}

// Done implements the Done interface of Augmenter.
func (ar *EnvAugmenter) Done() error {
	return nil
}
